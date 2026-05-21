package tui

import (
	"context"
	"fmt"
	"strings"

	"highway/core"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
)

func cloneTUIBatches(batches []core.ArtifactBatch) []core.ArtifactBatch {
	if len(batches) == 0 {
		return nil
	}

	cloned := make([]core.ArtifactBatch, len(batches))
	for i, batch := range batches {
		cloned[i] = batch
		cloned[i].Steps = append([]core.BatchArtifactStep(nil), batch.Steps...)
	}
	return cloned
}

func (m *model) syncBatchesToCore() {
	m.syncSelectedBatchFromSteps()
	m.core.SetBatches(cloneTUIBatches(m.batches))
}

func (m *model) syncSelectedBatchFromSteps() {
	index := m.batchList.GlobalIndex()
	if index < 0 || index >= len(m.batches) {
		return
	}

	items := m.batchSteps.Items()
	steps := make([]core.BatchArtifactStep, 0, len(items))
	for idx, item := range items {
		stepItem, ok := item.(batchStepItem)
		if !ok {
			continue
		}
		steps = append(steps, core.BatchArtifactStep{
			ArtifactID: stepItem.ArtifactID,
			Order:      idx + 1,
			Enabled:    stepItem.Enabled,
		})
	}

	m.batches[index].Steps = steps
	m.rebuildBatchList()
}

func (m *model) loadBatchStepsFromSelection() {
	m.ensureBatchSelection()
	if len(m.batchCatalog.Items()) > 0 && m.batchCatalog.GlobalIndex() < 0 {
		m.batchCatalog.Select(0)
	}
	index := m.batchList.GlobalIndex()
	if index < 0 || index >= len(m.batches) {
		m.batchSteps = buildBatchSteps(core.ArtifactBatch{}, m.core.Snapshot().Artifacts)
		return
	}
	m.batchSteps = buildBatchSteps(m.batches[index], m.core.Snapshot().Artifacts)
	if len(m.batchSteps.Items()) > 0 && m.batchSteps.GlobalIndex() < 0 {
		m.batchSteps.Select(0)
	}
	m.resizeViews()
}

func (m *model) rebuildBatchList() {
	data := m.core.Snapshot()
	data.Batches = cloneTUIBatches(m.batches)
	selected := m.batchList.GlobalIndex()
	m.batchList = buildBatchList(data)
	m.batchCatalog = buildArtifactList(data)
	if len(m.batches) > 0 {
		if selected < 0 {
			selected = 0
		}
		if selected >= len(m.batches) {
			selected = len(m.batches) - 1
		}
		m.batchList.Select(selected)
	}
	m.resizeViews()
}

func (m *model) createBatchFromArtifacts() bool {
	items := m.artifacts.Items()
	if len(items) == 0 {
		return false
	}

	steps := make([]core.BatchArtifactStep, 0, len(items))
	for idx, item := range items {
		artifactItem, ok := item.(sqlArtifactItem)
		if !ok {
			continue
		}
		steps = append(steps, core.BatchArtifactStep{
			ArtifactID: artifactItem.ID,
			Order:      idx + 1,
			Enabled:    true,
		})
	}

	m.initActiveConnection()
	batchID := m.nextBatchID()

	m.batches = append(m.batches, core.ArtifactBatch{
		ID:           batchID,
		Name:         fmt.Sprintf("batch-%d", batchID),
		Description:  "Nuevo escenario de ejecucion",
		ConnectionID: m.activeConnectionID,
		Steps:        steps,
	})
	m.rebuildBatchList()
	m.batchList.Select(len(m.batches) - 1)
	m.loadBatchStepsFromSelection()
	m.syncBatchesToCore()
	return true
}

func (m model) nextBatchID() int {
	maxID := 0
	for _, batch := range m.batches {
		if batch.ID > maxID {
			maxID = batch.ID
		}
	}
	return maxID + 1
}

func (m *model) removeSelectedBatch() bool {
	index := m.batchList.GlobalIndex()
	if index < 0 || index >= len(m.batches) {
		return false
	}
	m.batches = append(m.batches[:index], m.batches[index+1:]...)
	m.rebuildBatchList()
	if len(m.batches) > 0 {
		m.batchList.Select(min(index, len(m.batches)-1))
		m.loadBatchStepsFromSelection()
	}
	m.syncBatchesToCore()
	return true
}

func (m *model) toggleSelectedBatchStep() bool {
	index := m.batchSteps.GlobalIndex()
	if index < 0 || index >= len(m.batchSteps.Items()) {
		return false
	}
	stepItem, ok := m.batchSteps.Items()[index].(batchStepItem)
	if !ok {
		return false
	}
	stepItem.Enabled = !stepItem.Enabled
	_ = m.batchSteps.SetItem(index, stepItem)
	m.syncSelectedBatchFromSteps()
	m.syncBatchesToCore()
	return true
}

func (m *model) moveSelectedBatchStep(direction int) bool {
	items := m.batchSteps.Items()
	if len(items) < 2 {
		return false
	}

	index := m.batchSteps.GlobalIndex()
	target := index + direction
	if index < 0 || target < 0 || index >= len(items) || target >= len(items) {
		return false
	}

	items[index], items[target] = items[target], items[index]
	updated := make([]list.Item, 0, len(items))
	for idx, item := range items {
		stepItem, ok := item.(batchStepItem)
		if !ok {
			continue
		}
		stepItem.Order = idx + 1
		updated = append(updated, stepItem)
	}
	_ = m.batchSteps.SetItems(updated)
	m.batchSteps.Select(target)
	m.syncSelectedBatchFromSteps()
	m.syncBatchesToCore()
	return true
}

func (m model) connectionIDs() []int {
	items := m.connectionList.Items()
	ids := make([]int, 0, len(items))
	for _, item := range items {
		if connItem, ok := item.(connectionListItem); ok {
			ids = append(ids, connItem.ID)
		}
	}
	return ids
}

func (m model) connectionNameByID(id int) string {
	for _, item := range m.connectionList.Items() {
		if connItem, ok := item.(connectionListItem); ok && connItem.ID == id {
			return connItem.Name
		}
	}
	return ""
}

func (m model) connectionHostByID(id int) string {
	for _, item := range m.connectionList.Items() {
		if connItem, ok := item.(connectionListItem); ok && connItem.ID == id {
			return connItem.Host
		}
	}
	return ""
}

// connectionLabelByID returns a label that uniquely identifies the connection
// (id + target) so it is distinguishable even when names are duplicated.
func (m model) connectionLabelByID(id int) string {
	for _, item := range m.connectionList.Items() {
		connItem, ok := item.(connectionListItem)
		if !ok || connItem.ID != id {
			continue
		}
		target := connItem.Host
		if connItem.DBName != "" {
			target += "/" + connItem.DBName
		}
		label := fmt.Sprintf("%s (#%d)", connItem.Name, connItem.ID)
		if strings.TrimSpace(target) != "" {
			label += " · " + target
		}
		return label
	}
	return ""
}

// initActiveConnection picks a sensible active connection: keep the current one
// if still valid, otherwise fall back to the first batch's connection or the
// first available connection.
func (m *model) initActiveConnection() {
	ids := m.connectionIDs()
	if len(ids) == 0 {
		m.activeConnectionID = 0
		return
	}
	for _, id := range ids {
		if id == m.activeConnectionID {
			return
		}
	}
	if len(m.batches) > 0 && m.batches[0].ConnectionID != 0 {
		m.activeConnectionID = m.batches[0].ConnectionID
		return
	}
	m.activeConnectionID = ids[0]
}

// applyActiveConnectionToBatches makes every batch use the active connection so
// there is a single, global connection in effect.
func (m *model) applyActiveConnectionToBatches() {
	for i := range m.batches {
		m.batches[i].ConnectionID = m.activeConnectionID
	}
	selected := m.batchList.GlobalIndex()
	m.rebuildBatchList()
	if selected >= 0 && selected < len(m.batches) {
		m.batchList.Select(selected)
	}
	m.syncBatchesToCore()
}

func (m *model) cycleActiveConnection() bool {
	ids := m.connectionIDs()
	if len(ids) == 0 {
		return false
	}

	next := ids[0]
	for idx, id := range ids {
		if id == m.activeConnectionID {
			next = ids[(idx+1)%len(ids)]
			break
		}
	}
	m.activeConnectionID = next
	m.applyActiveConnectionToBatches()
	return true
}

func (m *model) resetSelectedBatchFromArtifacts() bool {
	index := m.batchList.GlobalIndex()
	if index < 0 || index >= len(m.batches) {
		return false
	}

	items := m.artifacts.Items()
	steps := make([]core.BatchArtifactStep, 0, len(items))
	for idx, item := range items {
		artifactItem, ok := item.(sqlArtifactItem)
		if !ok {
			continue
		}
		steps = append(steps, core.BatchArtifactStep{
			ArtifactID: artifactItem.ID,
			Order:      idx + 1,
			Enabled:    true,
		})
	}
	m.batches[index].Steps = steps
	m.loadBatchStepsFromSelection()
	m.syncBatchesToCore()
	return true
}

func (m *model) addSelectedArtifactToBatch() bool {
	batchIndex := m.batchList.GlobalIndex()
	if batchIndex < 0 || batchIndex >= len(m.batches) {
		return false
	}

	artifactIndex := m.batchCatalog.GlobalIndex()
	if artifactIndex < 0 || artifactIndex >= len(m.batchCatalog.Items()) {
		return false
	}

	artifactItem, ok := m.batchCatalog.Items()[artifactIndex].(sqlArtifactItem)
	if !ok {
		return false
	}

	for _, step := range m.batches[batchIndex].Steps {
		if step.ArtifactID == artifactItem.ID {
			return false
		}
	}

	m.batches[batchIndex].Steps = append(m.batches[batchIndex].Steps, core.BatchArtifactStep{
		ArtifactID: artifactItem.ID,
		Order:      len(m.batches[batchIndex].Steps) + 1,
		Enabled:    true,
	})
	m.loadBatchStepsFromSelection()
	if len(m.batchSteps.Items()) > 0 {
		m.batchSteps.Select(len(m.batchSteps.Items()) - 1)
	}
	m.syncBatchesToCore()
	return true
}

func (m *model) removeSelectedBatchStep() bool {
	batchIndex := m.batchList.GlobalIndex()
	if batchIndex < 0 || batchIndex >= len(m.batches) {
		return false
	}

	stepIndex := m.batchSteps.GlobalIndex()
	if stepIndex < 0 || stepIndex >= len(m.batches[batchIndex].Steps) {
		return false
	}

	steps := m.batches[batchIndex].Steps
	m.batches[batchIndex].Steps = append(steps[:stepIndex], steps[stepIndex+1:]...)
	for idx := range m.batches[batchIndex].Steps {
		m.batches[batchIndex].Steps[idx].Order = idx + 1
	}
	m.loadBatchStepsFromSelection()
	if len(m.batchSteps.Items()) > 0 {
		m.batchSteps.Select(min(stepIndex, len(m.batchSteps.Items())-1))
	}
	m.syncBatchesToCore()
	return true
}

func (m model) selectedBatchID() int {
	index := m.batchList.GlobalIndex()
	if index < 0 || index >= len(m.batches) {
		return 0
	}
	return m.batches[index].ID
}

// openBatchConfirm syncs the batch to core, scans its enabled SQL for
// destructive operations and opens the confirmation modal. The scan result
// drives whether a simple "y" suffices or the full word "yes" is required.
func (m *model) openBatchConfirm() (string, bool) {
	m.syncBatchesToCore()
	dangers, err := m.core.ScanBatchDangers(m.selectedBatchID())
	if err != nil {
		return "No se pudo analizar el batch: " + err.Error(), false
	}
	m.batchDangers = dangers
	m.batchConfirmInput = ""
	connectionID := m.activeConnectionID
	if index := m.batchList.GlobalIndex(); index >= 0 && index < len(m.batches) {
		connectionID = m.batches[index].ConnectionID
	}
	m.batchConfirmRemote = !core.IsLocalHost(m.connectionHostByID(connectionID))
	// For a remote target the user must type "yes <connection name>" so they
	// confirm exactly which database they are about to hit.
	connName := strings.TrimSpace(m.connectionNameByID(connectionID))
	if m.batchConfirmRemote && connName != "" {
		m.batchConfirmPhrase = "yes " + connName
	} else {
		m.batchConfirmPhrase = "yes"
	}
	m.batchConfirmOpen = true
	return "", true
}

// confirmPhraseMatches compares the typed confirmation against the required
// phrase, ignoring case and extra whitespace.
func confirmPhraseMatches(input, phrase string) bool {
	norm := func(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }
	return norm(input) == norm(phrase)
}

func (m model) startBatchRun() (tea.Model, tea.Cmd) {
	m.batchConfirmOpen = false
	m.batchConfirmInput = ""
	m.syncBatchesToCore()
	m.runningLabel = "Ejecutando batch..."
	return m.withStatus("Ejecutando batch..."), tea.Batch(m.spinner.Tick, m.runSelectedBatchCmd())
}

func (m model) runSelectedBatchCmd() tea.Cmd {
	batchID := m.selectedBatchID()
	return func() tea.Msg {
		if batchID == 0 {
			return batchRunResultMsg{err: fmt.Errorf("no hay batch seleccionado")}
		}
		err := m.core.ExecBatch(context.Background(), batchID)
		return batchRunResultMsg{err: err}
	}
}
