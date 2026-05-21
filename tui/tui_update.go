package tui

import (
	"strings"
	"time"

	"highway/core"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.runningLabel != "" {
		return m.updateRunning(msg)
	}
	if m.errorOpen {
		return m.updateErrorModal(msg)
	}
	if m.annotationOpen {
		return m.updateAnnotationModal(msg)
	}
	if m.batchConfirmOpen {
		return m.updateBatches(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeViews()
	case clearStatusMsg:
		m.status = ""
		return m, nil
	case projectCreateResultMsg:
		if msg.err != nil {
			return m.withStatus("Error creando proyecto: " + msg.err.Error()), clearStatusAfter(4 * time.Second)
		}
		m.projectPath = msg.path
		m.projectLoaded = true
		m.closeStartModals()
		m.screen = screenProject
		return m.withStatus("Proyecto creado en " + msg.path), clearStatusAfter(3 * time.Second)
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			// On the raw SQL editor "q" is a valid character to type, so let
			// it fall through to the screen handler instead of quitting.
			if m.screen == screenRawSQL {
				break
			}
			return m, tea.Quit
		case "ctrl+s":
			return m.saveState()
		case "ctrl+l":
			return m.loadStateAndGoProject()
		case "ctrl+tab", "ctrl+right":
			return m.navigateScreen(1)
		case "ctrl+shift+tab", "ctrl+left":
			return m.navigateScreen(-1)
		case "f1":
			return m.jumpToScreen(screenStart)
		case "f2":
			return m.jumpToScreen(screenProject)
		case "f3":
			return m.jumpToScreen(screenArtifacts)
		case "f4":
			return m.jumpToScreen(screenBatches)
		case "f5":
			return m.jumpToScreen(screenRawSQL)
		case "f6":
			return m.jumpToScreen(screenSummary)
		}
	}

	switch m.screen {
	case screenStart:
		return m.updateStart(msg)
	case screenProject:
		return m.updateProject(msg)
	case screenArtifacts:
		return m.updateArtifacts(msg)
	case screenBatches:
		return m.updateBatches(msg)
	case screenRawSQL:
		return m.updateRawSQL(msg)
	default:
		return m.updateSummary(msg)
	}
}

// updateRunning blocks all interaction while a batch or raw query is executing,
// keeping only the spinner animated and waiting for the result message.
func (m model) updateRunning(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeViews()
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case batchRunResultMsg:
		m.runningLabel = ""
		if msg.err != nil {
			m.openErrorModal("Error ejecutando batch", msg.err.Error())
			return m, nil
		}
		return m.withStatus("Batch ejecutado correctamente."), clearStatusAfter(2 * time.Second)
	case rawSQLResultMsg:
		m.runningLabel = ""
		if msg.err != nil {
			m.rawResult.SetContent(errStyle.Width(m.rawResult.Width()).Render(msg.err.Error()))
			m.rawResult.GotoTop()
			return m.withStatus("Error ejecutando SQL."), clearStatusAfter(3 * time.Second)
		}
		m.rawResult.SetContent(msg.output)
		m.rawResult.GotoTop()
		return m.withStatus("SQL ejecutado correctamente."), clearStatusAfter(2 * time.Second)
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) updateRawSQL(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "ctrl+e":
			if strings.TrimSpace(m.rawInput.Value()) == "" {
				return m.withStatus("Escribe una consulta antes de ejecutar."), clearStatusAfter(3 * time.Second)
			}
			if m.rawConnID == 0 {
				return m.withStatus("No hay conexion. Agregalas en F2 (Proyecto)."), clearStatusAfter(3 * time.Second)
			}
			m.runningLabel = "Ejecutando SQL..."
			return m.withStatus("Ejecutando SQL..."), tea.Batch(m.spinner.Tick, m.runRawSQLCmd())
		case "ctrl+n":
			if m.cycleRawConnection() {
				return m.withStatus("Conexion: " + m.connectionLabelByID(m.rawConnID)), clearStatusAfter(2 * time.Second)
			}
			return m.withStatus("No hay conexiones. Agregalas en F2 (Proyecto)."), clearStatusAfter(3 * time.Second)
		case "tab":
			if m.rawFocus == focusRawEditor {
				m.setRawFocus(focusRawResult)
			} else {
				m.setRawFocus(focusRawEditor)
			}
			return m, nil
		}
	}

	if m.rawFocus == focusRawResult {
		var cmd tea.Cmd
		m.rawResult, cmd = m.rawResult.Update(msg)
		return m, cmd
	}

	var cmd tea.Cmd
	m.rawInput, cmd = m.rawInput.Update(msg)
	return m, cmd
}

func (m model) updateStart(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.projectNameModal {
		return m.updateProjectNameModal(msg)
	}
	if m.jsonPickerOpen {
		return m.updateJSONPicker(msg)
	}

	if keyMsg, ok := msg.(tea.KeyPressMsg); ok && keyMsg.String() == "enter" {
		selected, _ := m.startMenu.SelectedItem().(startOption)
		switch selected.title {
		case "Cargar desde JSON":
			m.openJSONPicker()
			return m.withStatus("Selecciona el archivo JSON del proyecto."), m.jsonPicker.Init()
		case "Crear nuevo proyecto":
			m.core.SetConnections(nil)
			m.core.SetArtifacts(nil)
			m.core.SetBatches(nil)
			m.applyData(core.AppData{})
			m.projectLoaded = false
			m.projectPath = ""
			m.openProjectNameModal()
			return m.withStatus("Escribe el nombre del proyecto para crear su JSON."), nil
		}
	}

	var cmd tea.Cmd
	m.startMenu, cmd = m.startMenu.Update(msg)
	return m, cmd
}

func (m model) updateProject(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "b":
			m.screen = screenStart
			return m, nil
		case "tab":
			if m.connectionFocus == focusConnectionList {
				m.setProjectFocus(focusConnectionForm)
			} else {
				m.setProjectFocus(focusConnectionList)
			}
			return m, nil
		case "shift+tab":
			if m.connectionFocus == focusConnectionForm {
				m.setProjectFocus(focusConnectionList)
			} else {
				m.setProjectFocus(focusConnectionForm)
			}
			return m, nil
		case "n":
			m.clearConnectionForm()
			return m.withStatus("Formulario limpio para una conexion nueva."), clearStatusAfter(2 * time.Second)
		case "g":
			status, ok := m.goToArtifacts()
			if !ok {
				return m.withStatus(status), clearStatusAfter(3 * time.Second)
			}
			return m.withStatus(status), clearStatusAfter(2 * time.Second)
		case "x", "delete":
			if m.connectionFocus == focusConnectionList && m.removeSelectedConnection() {
				return m.withStatus("Conexion removida del proyecto."), clearStatusAfter(2 * time.Second)
			}
		case "enter":
			if m.connectionFocus == focusConnectionList {
				m.loadSelectedConnectionIntoForm()
				return m.withStatus("Conexion cargada en el formulario."), clearStatusAfter(2 * time.Second)
			}
			if m.connectionInputIdx == len(m.connectionInputs)-1 {
				status, ok := m.saveConnectionFromForm()
				if !ok {
					return m.withStatus(status), clearStatusAfter(3 * time.Second)
				}
				return m.withStatus(status), clearStatusAfter(2 * time.Second)
			}
			m.setConnectionInputFocus(m.connectionInputIdx + 1)
			return m, nil
		case "down":
			if m.connectionFocus == focusConnectionForm {
				m.setConnectionInputFocus((m.connectionInputIdx + 1) % len(m.connectionInputs))
				return m, nil
			}
		case "up":
			if m.connectionFocus == focusConnectionForm {
				m.setConnectionInputFocus((m.connectionInputIdx - 1 + len(m.connectionInputs)) % len(m.connectionInputs))
				return m, nil
			}
		case "ctrl+a":
			status, ok := m.saveConnectionFromForm()
			if !ok {
				return m.withStatus(status), clearStatusAfter(3 * time.Second)
			}
			return m.withStatus(status), clearStatusAfter(2 * time.Second)
		}
	}

	if m.connectionFocus == focusConnectionList {
		var cmd tea.Cmd
		m.connectionList, cmd = m.connectionList.Update(msg)
		return m, cmd
	}

	cmds := make([]tea.Cmd, len(m.connectionInputs))
	for i := range m.connectionInputs {
		m.connectionInputs[i], cmds[i] = m.connectionInputs[i].Update(msg)
	}
	return m, tea.Batch(cmds...)
}

func (m model) updateArtifacts(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "b":
			m.screen = screenProject
			return m, nil
		case "tab", "shift+tab":
			m.toggleArtifactFocus()
			return m, nil
		case "enter":
			if m.artifactFocus == focusList {
				m.syncArtifactsToCore()
				return m.jumpToScreen(screenBatches)
			}
		case "a":
			if m.artifactFocus == focusList && m.openAnnotationModal() {
				return m, nil
			}
		case "x", "delete":
			if m.artifactFocus == focusList {
				m.removeSelectedArtifact()
				return m.withStatus("Script removido de la lista."), clearStatusAfter(2 * time.Second)
			}
		case "ctrl+up", "alt+k", "K":
			if m.artifactFocus == focusList {
				m.moveSelectedArtifact(-1)
				return m.withStatus("Script movido hacia arriba."), clearStatusAfter(2 * time.Second)
			}
		case "ctrl+down", "alt+j", "J":
			if m.artifactFocus == focusList {
				m.moveSelectedArtifact(1)
				return m.withStatus("Script movido hacia abajo."), clearStatusAfter(2 * time.Second)
			}
		}
	}

	switch m.artifactFocus {
	case focusPicker:
		var cmd tea.Cmd
		m.picker, cmd = m.picker.Update(msg)
		if didSelect, path := m.picker.DidSelectFile(msg); didSelect {
			if added := m.addSQLArtifact(path); added {
				return m.withStatus("SQL agregado a la lista."), tea.Batch(clearStatusAfter(2*time.Second), cmd)
			}
			return m.withStatus("Ese SQL ya estaba en la lista."), tea.Batch(clearStatusAfter(2*time.Second), cmd)
		}
		if didSelect, path := m.picker.DidSelectDisabledFile(msg); didSelect {
			return m.withStatus("Archivo invalido: " + path), tea.Batch(clearStatusAfter(2*time.Second), cmd)
		}
		return m, cmd
	case focusList:
		var cmd tea.Cmd
		m.artifacts, cmd = m.artifacts.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) updateSummary(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok && keyMsg.String() == "b" {
		m.screen = screenBatches
		return m, nil
	}
	return m, nil
}

func (m model) updateBatches(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.batchConfirmOpen {
		keyMsg, ok := msg.(tea.KeyPressMsg)
		if !ok {
			return m, nil
		}
		if keyMsg.String() == "esc" {
			m.batchConfirmOpen = false
			m.batchConfirmInput = ""
			return m.withStatus("Ejecucion cancelada."), clearStatusAfter(2 * time.Second)
		}

		// When the batch contains destructive statements or targets a remote
		// (non-localhost) database, a single "y" is not enough: the user must
		// type the full word "yes".
		if len(m.batchDangers) > 0 || m.batchConfirmRemote {
			switch keyMsg.String() {
			case "enter":
				if confirmPhraseMatches(m.batchConfirmInput, m.batchConfirmPhrase) {
					return m.startBatchRun()
				}
				return m.withStatus("Confirmacion incorrecta: escribe exactamente '" + m.batchConfirmPhrase + "'."), clearStatusAfter(4 * time.Second)
			case "backspace":
				if n := len(m.batchConfirmInput); n > 0 {
					m.batchConfirmInput = m.batchConfirmInput[:n-1]
				}
				return m, nil
			default:
				if len(keyMsg.String()) == 1 {
					m.batchConfirmInput += keyMsg.String()
				}
				return m, nil
			}
		}

		switch keyMsg.String() {
		case "y", "enter":
			return m.startBatchRun()
		case "n":
			m.batchConfirmOpen = false
			return m.withStatus("Ejecucion cancelada."), clearStatusAfter(2 * time.Second)
		}
		return m, nil
	}

	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "b":
			m.screen = screenArtifacts
			return m, nil
		case "tab":
			if m.batchFocus == focusBatchList {
				m.setBatchFocus(focusBatchCatalog)
			} else if m.batchFocus == focusBatchCatalog {
				m.setBatchFocus(focusBatchSteps)
			} else {
				m.setBatchFocus(focusBatchList)
			}
			return m, nil
		case "shift+tab":
			if m.batchFocus == focusBatchSteps {
				m.setBatchFocus(focusBatchCatalog)
			} else if m.batchFocus == focusBatchCatalog {
				m.setBatchFocus(focusBatchList)
			} else {
				m.setBatchFocus(focusBatchSteps)
			}
			return m, nil
		case "n":
			if m.createBatchFromArtifacts() {
				return m.withStatus("Batch creado desde el catalogo actual."), clearStatusAfter(2 * time.Second)
			}
			return m.withStatus("Agrega artifacts antes de crear un batch."), clearStatusAfter(3 * time.Second)
		case "x", "delete":
			if m.batchFocus == focusBatchList && m.removeSelectedBatch() {
				return m.withStatus("Batch removido."), clearStatusAfter(2 * time.Second)
			}
			if m.batchFocus == focusBatchSteps && m.removeSelectedBatchStep() {
				return m.withStatus("SQL removido del batch."), clearStatusAfter(2 * time.Second)
			}
		case "c":
			if m.cycleActiveConnection() {
				return m.withStatus("Conexion activa: " + m.connectionLabelByID(m.activeConnectionID)), clearStatusAfter(2 * time.Second)
			}
			return m.withStatus("No hay conexiones. Agregalas en F2 (Proyecto)."), clearStatusAfter(3 * time.Second)
		case "e":
			if m.batchFocus == focusBatchList || m.batchFocus == focusBatchSteps {
				if m.selectedBatchID() == 0 {
					return m.withStatus("Selecciona un batch para ejecutar."), clearStatusAfter(3 * time.Second)
				}
				if status, ok := m.openBatchConfirm(); !ok {
					return m.withStatus(status), clearStatusAfter(4 * time.Second)
				}
				return m, nil
			}
		case "r":
			if m.batchFocus == focusBatchSteps && m.resetSelectedBatchFromArtifacts() {
				return m.withStatus("Batch reconstruido desde el catalogo."), clearStatusAfter(2 * time.Second)
			}
		case " ":
			if m.batchFocus == focusBatchSteps && m.toggleSelectedBatchStep() {
				return m.withStatus("Paso del batch actualizado."), clearStatusAfter(2 * time.Second)
			}
		case "ctrl+up", "alt+k", "K":
			if m.batchFocus == focusBatchSteps && m.moveSelectedBatchStep(-1) {
				return m.withStatus("Paso movido hacia arriba."), clearStatusAfter(2 * time.Second)
			}
		case "ctrl+down", "alt+j", "J":
			if m.batchFocus == focusBatchSteps && m.moveSelectedBatchStep(1) {
				return m.withStatus("Paso movido hacia abajo."), clearStatusAfter(2 * time.Second)
			}
		case "enter":
			if m.batchFocus == focusBatchList {
				m.loadBatchStepsFromSelection()
				m.setBatchFocus(focusBatchCatalog)
				return m.withStatus("Batch cargado para editar su corrida."), clearStatusAfter(2 * time.Second)
			}
			if m.batchFocus == focusBatchCatalog {
				if m.addSelectedArtifactToBatch() {
					return m.withStatus("SQL agregado. Sigue eligiendo o Tab para ordenar."), clearStatusAfter(2 * time.Second)
				}
				return m.withStatus("Ese SQL ya estaba en el batch o no habia seleccion."), clearStatusAfter(2 * time.Second)
			}
			m.syncBatchesToCore()
			m.screen = screenSummary
			return m, nil
		}
	}

	if m.batchFocus == focusBatchList {
		oldIndex := m.batchList.GlobalIndex()
		var cmd tea.Cmd
		m.batchList, cmd = m.batchList.Update(msg)
		if m.batchList.GlobalIndex() != oldIndex {
			m.loadBatchStepsFromSelection()
		}
		return m, cmd
	}

	if m.batchFocus == focusBatchCatalog {
		var cmd tea.Cmd
		m.batchCatalog, cmd = m.batchCatalog.Update(msg)
		return m, cmd
	}

	var cmd tea.Cmd
	m.batchSteps, cmd = m.batchSteps.Update(msg)
	return m, cmd
}

func (m model) saveState() (tea.Model, tea.Cmd) {
	if !m.projectLoaded || strings.TrimSpace(m.projectPath) == "" {
		return m.withStatus("No hay proyecto cargado para guardar."), clearStatusAfter(3 * time.Second)
	}
	if m.screen == screenProject {
		status, ok := m.saveConnectionDraftIfNeeded()
		if !ok {
			return m.withStatus(status), clearStatusAfter(3 * time.Second)
		}
	}
	m.syncCurrentScreenToCore()
	if err := m.core.Save(m.projectPath); err != nil {
		return m.withStatus("Error guardando JSON: " + err.Error()), clearStatusAfter(3 * time.Second)
	}
	return m.withStatus("Estado guardado en " + m.projectPath), clearStatusAfter(2 * time.Second)
}

func (m model) updateErrorModal(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeViews()
		return m, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc", "enter", "q":
			m.errorOpen = false
			return m, nil
		case "ctrl+c":
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.errorView, cmd = m.errorView.Update(msg)
	return m, cmd
}

func (m model) updateAnnotationModal(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "esc":
			m.closeAnnotationModal()
			return m.withStatus("Edicion de anotacion cancelada."), clearStatusAfter(2 * time.Second)
		case "ctrl+s":
			if m.saveAnnotationModal() {
				return m.saveState()
			}
		case "ctrl+enter":
			if m.saveAnnotationModal() {
				return m.withStatus("Anotacion guardada en el SQL."), clearStatusAfter(2 * time.Second)
			}
		}
	}

	var cmd tea.Cmd
	m.notes, cmd = m.notes.Update(msg)
	return m, cmd
}

func (m model) loadStateAndGoProject() (tea.Model, tea.Cmd) {
	if !m.projectLoaded || strings.TrimSpace(m.projectPath) == "" {
		m.openJSONPicker()
		return m.withStatus("Selecciona un JSON para cargar el proyecto."), m.jsonPicker.Init()
	}

	if err := m.core.Load(m.projectPath); err != nil {
		return m.withStatus("Error cargando JSON: " + err.Error()), clearStatusAfter(3 * time.Second)
	}

	m.applyData(m.core.Snapshot())
	if m.screen == screenStart {
		m.screen = screenProject
	}
	return m.withStatus("Estado cargado desde " + m.projectPath), tea.Batch(clearStatusAfter(2*time.Second), m.picker.Init())
}

func (m model) updateProjectNameModal(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "esc":
			m.closeStartModals()
			return m.withStatus("Creacion de proyecto cancelada."), clearStatusAfter(2 * time.Second)
		case "enter":
			name := strings.TrimSpace(m.projectNameInput.Value())
			if name == "" {
				return m.withStatus("El nombre del proyecto es obligatorio."), clearStatusAfter(3 * time.Second)
			}
			return m.withStatus("Creando proyecto..."), m.createProjectCmd(name)
		}
	}

	var cmd tea.Cmd
	m.projectNameInput, cmd = m.projectNameInput.Update(msg)
	return m, cmd
}

func (m model) updateJSONPicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok && keyMsg.String() == "esc" {
		m.closeStartModals()
		return m.withStatus("Carga de proyecto cancelada."), clearStatusAfter(2 * time.Second)
	}

	var cmd tea.Cmd
	m.jsonPicker, cmd = m.jsonPicker.Update(msg)
	if didSelect, path := m.jsonPicker.DidSelectFile(msg); didSelect {
		if err := m.core.Load(path); err != nil {
			m.openErrorModal("Error cargando JSON", err.Error())
			return m, nil
		}
		m.projectPath = path
		m.applyData(m.core.Snapshot())
		m.closeStartModals()
		m.screen = screenProject
		return m.withStatus("Proyecto cargado desde " + path), tea.Batch(clearStatusAfter(2*time.Second), m.picker.Init())
	}
	if didSelect, path := m.jsonPicker.DidSelectDisabledFile(msg); didSelect {
		return m.withStatus("Archivo invalido: " + path), clearStatusAfter(2 * time.Second)
	}
	return m, cmd
}

func (m model) navigateScreen(direction int) (tea.Model, tea.Cmd) {
	order := []screen{screenStart, screenProject, screenArtifacts, screenBatches, screenRawSQL, screenSummary}
	current := 0
	for idx, candidate := range order {
		if candidate == m.screen {
			current = idx
			break
		}
	}

	next := (current + direction + len(order)) % len(order)
	return m.jumpToScreen(order[next])
}

func (m model) jumpToScreen(target screen) (tea.Model, tea.Cmd) {
	status, ok := m.selectScreen(target)
	if !ok {
		if status == "" {
			status = "No se pudo abrir esa pestana."
		}
		return m.withStatus(status), clearStatusAfter(3 * time.Second)
	}
	if status == "" {
		return m, nil
	}
	return m.withStatus(status), clearStatusAfter(2 * time.Second)
}
