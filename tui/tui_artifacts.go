package tui

import (
	"path/filepath"

	"highway/core"

	"charm.land/bubbles/v2/list"
)

func (m *model) syncArtifactsToCore() {
	if m.annotationOpen {
		m.syncAnnotationToCurrentSelection()
	}
	items := m.artifacts.Items()
	artifacts := make([]core.Artifact, 0, len(items))
	for idx, item := range items {
		sqlItem, ok := item.(sqlArtifactItem)
		if !ok {
			continue
		}

		artifacts = append(artifacts, core.Artifact{
			ID:         sqlItem.ID,
			Order:      idx + 1,
			Name:       sqlItem.Name,
			Path:       sqlItem.Path,
			Annotation: sqlItem.Annotation,
		})
	}
	m.core.SetArtifacts(artifacts)
	snapshot := m.core.Snapshot()
	m.batches = cloneTUIBatches(snapshot.Batches)
	m.rebuildBatchList()
	m.batchCatalog = buildArtifactList(snapshot)
	if len(m.batches) > 0 {
		current := min(max(m.batchList.GlobalIndex(), 0), len(m.batches)-1)
		m.batchList.Select(current)
		m.loadBatchStepsFromSelection()
	}
}

func (m *model) currentSelectedIndex() int {
	index := m.artifacts.GlobalIndex()
	if index < 0 || index >= len(m.artifacts.Items()) {
		return -1
	}
	return index
}

func (m *model) loadAnnotationFromSelection() {
	index := m.currentSelectedIndex()
	if index == -1 {
		m.notes.SetValue("")
		return
	}

	item, ok := m.artifacts.Items()[index].(sqlArtifactItem)
	if !ok {
		m.notes.SetValue("")
		return
	}
	m.notes.SetValue(item.Annotation)
}

func (m *model) syncAnnotationToCurrentSelection() {
	index := m.currentSelectedIndex()
	if index == -1 {
		return
	}

	item, ok := m.artifacts.Items()[index].(sqlArtifactItem)
	if !ok {
		return
	}

	item.Annotation = m.notes.Value()
	_ = m.artifacts.SetItem(index, item)
}

func (m *model) saveAnnotationModal() bool {
	if m.currentSelectedIndex() == -1 {
		return false
	}
	m.syncAnnotationToCurrentSelection()
	m.syncArtifactsToCore()
	m.closeAnnotationModal()
	return true
}

func (m *model) addSQLArtifact(path string) bool {
	cleanPath := filepath.Clean(path)
	for _, item := range m.artifacts.Items() {
		sqlItem, ok := item.(sqlArtifactItem)
		if ok && filepath.Clean(sqlItem.Path) == cleanPath {
			return false
		}
	}

	nextID := m.core.Snapshot().MaxArtifactID + 1
	order := len(m.artifacts.Items()) + 1
	item := sqlArtifactItem{
		ID:         nextID,
		Order:      order,
		Name:       filepath.Base(cleanPath),
		Path:       cleanPath,
		Annotation: "",
	}

	_ = m.artifacts.InsertItem(order-1, item)
	m.artifacts.Select(order - 1)
	m.loadAnnotationFromSelection()
	m.syncArtifactsToCore()
	return true
}

func (m *model) moveSelectedArtifact(direction int) {
	m.syncAnnotationToCurrentSelection()

	items := m.artifacts.Items()
	if len(items) < 2 {
		return
	}

	index := m.artifacts.GlobalIndex()
	target := index + direction
	if index < 0 || index >= len(items) || target < 0 || target >= len(items) {
		return
	}

	items[index], items[target] = items[target], items[index]

	updated := make([]list.Item, 0, len(items))
	for idx, item := range items {
		sqlItem, ok := item.(sqlArtifactItem)
		if !ok {
			continue
		}
		sqlItem.Order = idx + 1
		updated = append(updated, sqlItem)
	}

	_ = m.artifacts.SetItems(updated)
	m.artifacts.Select(target)
	m.loadAnnotationFromSelection()
	m.syncArtifactsToCore()
}

func (m *model) removeSelectedArtifact() {
	m.syncAnnotationToCurrentSelection()

	index := m.artifacts.GlobalIndex()
	if index < 0 || index >= len(m.artifacts.Items()) {
		return
	}

	m.artifacts.RemoveItem(index)
	items := m.artifacts.Items()
	updated := make([]list.Item, 0, len(items))
	for idx, item := range items {
		sqlItem, ok := item.(sqlArtifactItem)
		if !ok {
			continue
		}
		sqlItem.Order = idx + 1
		updated = append(updated, sqlItem)
	}

	_ = m.artifacts.SetItems(updated)
	if len(updated) > 0 {
		m.artifacts.Select(min(index, len(updated)-1))
	} else {
		m.setArtifactFocus(focusPicker)
	}
	m.loadAnnotationFromSelection()
	m.syncArtifactsToCore()
}
