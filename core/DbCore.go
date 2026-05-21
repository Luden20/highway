package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
)

// DangerStat counts how many times a destructive operation appears in a batch.
type DangerStat struct {
	Label string
	Count int
}

// dangerPattern pairs a human label with a regexp matching a destructive SQL
// operation. Order defines display order in the confirmation modal.
type dangerPattern struct {
	label string
	re    *regexp.Regexp
}

// dangerPatterns are counted per statement. Order defines display order.
var dangerPatterns = []dangerPattern{
	{"DROP DATABASE", regexp.MustCompile(`(?i)\bdrop\s+database\b`)},
	{"DROP SCHEMA", regexp.MustCompile(`(?i)\bdrop\s+schema\b`)},
	{"DROP TABLE", regexp.MustCompile(`(?i)\bdrop\s+table\b`)},
	{"DROP MATERIALIZED VIEW", regexp.MustCompile(`(?i)\bdrop\s+materialized\s+view\b`)},
	{"DROP VIEW", regexp.MustCompile(`(?i)\bdrop\s+view\b`)},
	{"DROP INDEX", regexp.MustCompile(`(?i)\bdrop\s+index\b`)},
	{"DROP COLUMN", regexp.MustCompile(`(?i)\bdrop\s+column\b`)},
	{"ALTER TABLE", regexp.MustCompile(`(?i)\balter\s+table\b`)},
	{"TRUNCATE", regexp.MustCompile(`(?i)\btruncate\b`)},
	{"DELETE", regexp.MustCompile(`(?i)\bdelete\s+from\b`)},
	{"UPDATE", regexp.MustCompile(`(?i)\bupdate\b`)},
}

// Per-statement detectors for the most dangerous case: a DELETE/UPDATE that
// touches every row because it has no WHERE clause.
var (
	deleteStmtRe = regexp.MustCompile(`(?i)\bdelete\s+from\b`)
	updateStmtRe = regexp.MustCompile(`(?i)\bupdate\b\s+\S+\s+\bset\b`)
	whereRe      = regexp.MustCompile(`(?i)\bwhere\b`)
)

// dangerOrder is the full display order including the synthetic "sin WHERE"
// rows that ScanBatchDangers computes per statement.
var dangerOrder = []string{
	"DROP DATABASE",
	"DROP SCHEMA",
	"DROP TABLE",
	"DROP MATERIALIZED VIEW",
	"DROP VIEW",
	"DROP INDEX",
	"DROP COLUMN",
	"ALTER TABLE",
	"TRUNCATE",
	"DELETE sin WHERE",
	"DELETE",
	"UPDATE sin WHERE",
	"UPDATE",
}

// stripSQLComments removes -- line comments and /* */ block comments so that
// commented-out destructive statements are not counted.
var (
	lineCommentRe  = regexp.MustCompile(`--[^\n]*`)
	blockCommentRe = regexp.MustCompile(`(?s)/\*.*?\*/`)
)

func stripSQLComments(sql string) string {
	sql = blockCommentRe.ReplaceAllString(sql, " ")
	sql = lineCommentRe.ReplaceAllString(sql, " ")
	return sql
}

// IsLocalHost reports whether a connection host points at the local machine.
// Anything else is treated as remote and warrants an extra warning.
func IsLocalHost(host string) bool {
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "", "localhost", "127.0.0.1", "::1", "0.0.0.0":
		return true
	}
	return false
}

// ScanBatchDangers reads the SQL of every enabled step in a batch and counts
// destructive operations (DROP/TRUNCATE/DELETE), returning only the categories
// that actually appear, in severity order.
func (a *AppCore) ScanBatchDangers(batchID int) ([]DangerStat, error) {
	batch := a.GetBatch(batchID)
	if batch == nil {
		return nil, errors.New("batch not found")
	}

	counts := make(map[string]int, len(dangerOrder))
	for _, step := range batch.Steps {
		if !step.Enabled {
			continue
		}
		artifact := a.GetArtifact(step.ArtifactID)
		if artifact == nil {
			continue
		}
		sqlBytes, readErr := os.ReadFile(a.ResolvePath(artifact.Path))
		if readErr != nil {
			return nil, fmt.Errorf("no se pudo leer %s: %w", artifact.Path, readErr)
		}

		sql := stripSQLComments(string(sqlBytes))
		for _, statement := range strings.Split(sql, ";") {
			stmt := strings.TrimSpace(statement)
			if stmt == "" {
				continue
			}
			for _, pattern := range dangerPatterns {
				counts[pattern.label] += len(pattern.re.FindAllStringIndex(stmt, -1))
			}
			hasWhere := whereRe.MatchString(stmt)
			if deleteStmtRe.MatchString(stmt) && !hasWhere {
				counts["DELETE sin WHERE"]++
			}
			if updateStmtRe.MatchString(stmt) && !hasWhere {
				counts["UPDATE sin WHERE"]++
			}
		}
	}

	stats := make([]DangerStat, 0, len(dangerOrder))
	for _, label := range dangerOrder {
		if n := counts[label]; n > 0 {
			stats = append(stats, DangerStat{Label: label, Count: n})
		}
	}
	return stats, nil
}

func (a *AppCore) BuildConnection(ctx context.Context, id int) (*pgx.Conn, error) {
	conn := a.GetConnection(id)
	if conn == nil {
		return nil, errors.New("connection not found")
	}
	dbConn, err := pgx.Connect(ctx, conn.BuildPostgresConnString())
	if err != nil {
		return nil, fmt.Errorf("error connecting to database: %w", err)
	}
	return dbConn, nil
}

func (a *AppCore) ExecBatch(ctx context.Context, batchID int) error {
	batch := a.GetBatch(batchID)
	if batch == nil {
		return errors.New("batch not found")
	}
	if batch.ConnectionID == 0 {
		return errors.New("batch has no connection assigned")
	}

	conn, err := a.BuildConnection(ctx, batch.ConnectionID)
	if err != nil {
		return err
	}
	defer func(conn *pgx.Conn, ctx context.Context) {
		closeErr := conn.Close(ctx)
		if closeErr != nil {
			fmt.Println(closeErr)
		}
	}(conn, ctx)

	for _, step := range batch.Steps {
		if !step.Enabled {
			continue
		}

		artifact := a.GetArtifact(step.ArtifactID)
		if artifact == nil {
			return fmt.Errorf("artifact %d not found in batch %d", step.ArtifactID, batch.ID)
		}

		sqlBytes, readErr := os.ReadFile(a.ResolvePath(artifact.Path))
		if readErr != nil {
			return readErr
		}
		if _, execErr := conn.Exec(ctx, string(sqlBytes)); execErr != nil {
			return fmt.Errorf("error executing artifact %s in batch %s: %w", artifact.Name, batch.Name, execErr)
		}
	}

	return nil
}

// ExecRaw runs an arbitrary SQL statement against the given connection and
// returns a human-readable rendering of the result: a column-aligned table for
// queries that return rows, or a command tag (e.g. "INSERT 0 3") otherwise.
func (a *AppCore) ExecRaw(ctx context.Context, connectionID int, sql string) (string, error) {
	if strings.TrimSpace(sql) == "" {
		return "", errors.New("la consulta esta vacia")
	}
	if connectionID == 0 {
		return "", errors.New("no hay conexion seleccionada")
	}

	conn, err := a.BuildConnection(ctx, connectionID)
	if err != nil {
		return "", err
	}
	defer func(conn *pgx.Conn, ctx context.Context) {
		if closeErr := conn.Close(ctx); closeErr != nil {
			fmt.Println(closeErr)
		}
	}(conn, ctx)

	rows, err := conn.Query(ctx, sql)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	fields := rows.FieldDescriptions()
	headers := make([]string, len(fields))
	for i, field := range fields {
		headers[i] = string(field.Name)
	}

	var records [][]string
	for rows.Next() {
		values, valErr := rows.Values()
		if valErr != nil {
			return "", valErr
		}
		record := make([]string, len(values))
		for i, value := range values {
			if value == nil {
				record[i] = "NULL"
				continue
			}
			record[i] = fmt.Sprintf("%v", value)
		}
		records = append(records, record)
	}
	if rows.Err() != nil {
		return "", rows.Err()
	}

	tag := rows.CommandTag()
	if len(headers) == 0 {
		return tag.String(), nil
	}

	table := renderTable(headers, records)
	return fmt.Sprintf("%s\n\n%s (%d filas)", table, tag.String(), len(records)), nil
}

// renderTable lays out headers and records into a simple aligned ASCII table.
func renderTable(headers []string, records [][]string) string {
	widths := make([]int, len(headers))
	for i, header := range headers {
		widths[i] = len(header)
	}
	for _, record := range records {
		for i, cell := range record {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	pad := func(value string, width int) string {
		if len(value) >= width {
			return value
		}
		return value + strings.Repeat(" ", width-len(value))
	}

	var b strings.Builder
	for i, header := range headers {
		if i > 0 {
			b.WriteString(" | ")
		}
		b.WriteString(pad(header, widths[i]))
	}
	b.WriteString("\n")
	for i, width := range widths {
		if i > 0 {
			b.WriteString("-+-")
		}
		b.WriteString(strings.Repeat("-", width))
	}
	if len(records) == 0 {
		b.WriteString("\n(sin filas)")
		return b.String()
	}
	for _, record := range records {
		b.WriteString("\n")
		for i := range headers {
			if i > 0 {
				b.WriteString(" | ")
			}
			cell := ""
			if i < len(record) {
				cell = record[i]
			}
			b.WriteString(pad(cell, widths[i]))
		}
	}
	return b.String()
}
