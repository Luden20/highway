package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
)

type AppData struct {
	Connections   []DbConnection  `json:"connections"`
	Artifacts     []Artifact      `json:"artifacts"`
	MaxArtifactID int             `json:"maxArtifactId"`
	Batches       []ArtifactBatch `json:"batches"`
	MaxBatchID    int             `json:"maxBatchId"`
}

type ArtifactBatch struct {
	ID           int                 `json:"id"`
	Name         string              `json:"name"`
	Description  string              `json:"description"`
	ConnectionID int                 `json:"connectionId"`
	Steps        []BatchArtifactStep `json:"steps"`
}

type BatchArtifactStep struct {
	ArtifactID int  `json:"artifactId"`
	Order      int  `json:"order"`
	Enabled    bool `json:"enabled"`
}

type Artifact struct {
	ID         int    `json:"id"`
	Order      int    `json:"order"`
	Name       string `json:"name"`
	Path       string `json:"path"`
	Annotation string `json:"annotation"`
}

type DbConnection struct {
	Id       int    `json:"id"`
	Name     string `json:"name"`
	Host     string `json:"host"`
	Port     string `json:"port"`
	DBName   string `json:"dbName"`
	User     string `json:"user"`
	Password string `json:"password"`
}

type AppCore struct {
	mu sync.RWMutex
	// baseDir is the directory of the loaded/saved project JSON. Relative
	// artifact paths are resolved against it.
	baseDir string
	Data    AppData
}

var (
	instance *AppCore
	once     sync.Once
)

func GetAppCore() *AppCore {
	once.Do(func() {
		instance = &AppCore{}
	})
	return instance
}

func (a *AppCore) Load(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var data AppData
	if err := json.Unmarshal(raw, &data); err != nil {
		return err
	}

	data.Connections = normalizeConnections(data.Connections)
	data.Artifacts = normalizeArtifacts(data.Artifacts)
	data.Batches = normalizeBatches(data.Batches, data.Artifacts, data.Connections)
	data.MaxArtifactID = maxArtifactID(data.Artifacts, data.MaxArtifactID)
	data.MaxBatchID = maxBatchID(data.Batches, data.MaxBatchID)

	a.mu.Lock()
	a.Data = data
	a.baseDir = filepath.Dir(path)
	a.mu.Unlock()
	return nil
}

func (a *AppCore) Save(path string) error {
	a.mu.RLock()
	data := AppData{
		Connections:   normalizeConnections(a.Data.Connections),
		Artifacts:     normalizeArtifacts(a.Data.Artifacts),
		MaxArtifactID: maxArtifactID(a.Data.Artifacts, a.Data.MaxArtifactID),
		Batches:       normalizeBatches(a.Data.Batches, a.Data.Artifacts, a.Data.Connections),
		MaxBatchID:    maxBatchID(a.Data.Batches, a.Data.MaxBatchID),
	}
	a.mu.RUnlock()

	// Persist artifact paths relative to the project JSON so the project stays
	// portable. Paths that can't be made relative (e.g. a different drive) keep
	// their absolute form.
	baseDir := filepath.Dir(path)
	for i := range data.Artifacts {
		p := data.Artifacts[i].Path
		if p == "" || !filepath.IsAbs(p) {
			continue
		}
		if rel, relErr := filepath.Rel(baseDir, p); relErr == nil {
			data.Artifacts[i].Path = rel
		}
	}

	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return err
	}

	a.mu.Lock()
	a.baseDir = filepath.Dir(path)
	a.mu.Unlock()
	return nil
}

// ResolvePath turns a possibly-relative artifact path into an absolute one,
// resolving relative paths against the directory of the project JSON.
func (a *AppCore) ResolvePath(p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	a.mu.RLock()
	base := a.baseDir
	a.mu.RUnlock()
	if base == "" {
		return p
	}
	return filepath.Join(base, p)
}

func (a *AppCore) Snapshot() AppData {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return AppData{
		Connections:   slices.Clone(a.Data.Connections),
		Artifacts:     slices.Clone(a.Data.Artifacts),
		MaxArtifactID: a.Data.MaxArtifactID,
		Batches:       cloneBatches(a.Data.Batches),
		MaxBatchID:    a.Data.MaxBatchID,
	}
}

func (a *AppCore) SetConnections(connections []DbConnection) {
	a.mu.Lock()
	a.Data.Connections = normalizeConnections(connections)
	a.Data.Batches = normalizeBatches(a.Data.Batches, a.Data.Artifacts, a.Data.Connections)
	a.mu.Unlock()
}

func (a *AppCore) SetArtifacts(artifacts []Artifact) {
	a.mu.Lock()
	a.Data.Artifacts = normalizeArtifacts(artifacts)
	a.Data.MaxArtifactID = maxArtifactID(a.Data.Artifacts, a.Data.MaxArtifactID)
	a.Data.Batches = normalizeBatches(a.Data.Batches, a.Data.Artifacts, a.Data.Connections)
	a.mu.Unlock()
}

func (a *AppCore) SetBatches(batches []ArtifactBatch) {
	a.mu.Lock()
	a.Data.Batches = normalizeBatches(batches, a.Data.Artifacts, a.Data.Connections)
	a.Data.MaxBatchID = maxBatchID(a.Data.Batches, a.Data.MaxBatchID)
	a.mu.Unlock()
}

func normalizeConnections(connections []DbConnection) []DbConnection {
	if len(connections) == 0 {
		return nil
	}

	normalized := slices.Clone(connections)
	for idx := range normalized {
		if normalized[idx].Id == 0 {
			normalized[idx].Id = idx + 1
		}
	}
	return normalized
}

func normalizeArtifacts(artifacts []Artifact) []Artifact {
	if len(artifacts) == 0 {
		return nil
	}

	normalized := slices.Clone(artifacts)
	for idx := range normalized {
		normalized[idx].Order = idx + 1
	}
	return normalized
}

func normalizeBatches(batches []ArtifactBatch, artifacts []Artifact, connections []DbConnection) []ArtifactBatch {
	if len(batches) == 0 {
		return nil
	}

	artifactIDs := make(map[int]struct{}, len(artifacts))
	for _, artifact := range artifacts {
		artifactIDs[artifact.ID] = struct{}{}
	}

	connectionIDs := make(map[int]struct{}, len(connections))
	for _, connection := range connections {
		connectionIDs[connection.Id] = struct{}{}
	}

	normalized := make([]ArtifactBatch, 0, len(batches))
	for idx, batch := range batches {
		if batch.ID == 0 {
			batch.ID = idx + 1
		}
		if _, ok := connectionIDs[batch.ConnectionID]; !ok {
			batch.ConnectionID = 0
		}

		steps := make([]BatchArtifactStep, 0, len(batch.Steps))
		for _, step := range batch.Steps {
			if _, ok := artifactIDs[step.ArtifactID]; !ok {
				continue
			}
			steps = append(steps, BatchArtifactStep{
				ArtifactID: step.ArtifactID,
				Enabled:    step.Enabled,
			})
		}

		for stepIndex := range steps {
			steps[stepIndex].Order = stepIndex + 1
		}

		batch.Steps = steps
		normalized = append(normalized, batch)
	}

	return normalized
}

func cloneBatches(batches []ArtifactBatch) []ArtifactBatch {
	if len(batches) == 0 {
		return nil
	}

	cloned := make([]ArtifactBatch, len(batches))
	for i, batch := range batches {
		cloned[i] = batch
		cloned[i].Steps = slices.Clone(batch.Steps)
	}
	return cloned
}

func maxArtifactID(artifacts []Artifact, fallback int) int {
	maxID := fallback
	for _, artifact := range artifacts {
		if artifact.ID > maxID {
			maxID = artifact.ID
		}
	}
	return maxID
}

func maxBatchID(batches []ArtifactBatch, fallback int) int {
	maxID := fallback
	for _, batch := range batches {
		if batch.ID > maxID {
			maxID = batch.ID
		}
	}
	return maxID
}

func (a *AppCore) GetConnection(id int) *DbConnection {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, conn := range a.Data.Connections {
		if conn.Id == id {
			connCopy := conn
			return &connCopy
		}
	}
	return nil
}

func (a *AppCore) GetBatch(id int) *ArtifactBatch {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, batch := range a.Data.Batches {
		if batch.ID == id {
			batchCopy := batch
			batchCopy.Steps = slices.Clone(batch.Steps)
			return &batchCopy
		}
	}
	return nil
}

func (a *AppCore) GetArtifact(id int) *Artifact {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, artifact := range a.Data.Artifacts {
		if artifact.ID == id {
			artifactCopy := artifact
			return &artifactCopy
		}
	}
	return nil
}

func (conn *DbConnection) BuildPostgresConnString() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s", conn.User, conn.Password, conn.Host, conn.Port, conn.DBName)
}
