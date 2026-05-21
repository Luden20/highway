package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"highway/core"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

func (m *model) errorModalSize() (width, height int) {
	return min(max(60, m.width-12), 120), min(max(10, m.height-12), 40)
}

// openErrorModal shows the full error text in a scrollable popup.
func (m *model) openErrorModal(title, body string) {
	w, h := m.errorModalSize()
	vp := viewport.New()
	vp.SetWidth(w)
	vp.SetHeight(h)
	vp.SetContent(wrapText(body, w))
	vp.GotoTop()
	m.errorTitle = title
	m.errorBody = body
	m.errorView = vp
	m.errorOpen = true
}

func NewModel() model {
	appCore := core.GetAppCore()

	m := model{
		core:             appCore,
		screen:           screenStart,
		startMenu:        buildStartMenu(),
		jsonPicker:       buildJSONPicker(),
		projectNameInput: buildProjectNameInput(),
		connectionList:   buildConnectionList(core.AppData{}),
		connectionInputs: buildConnectionInputs(core.DbConnection{}),
		picker:           buildFilePicker(),
		artifacts:        buildArtifactList(core.AppData{}),
		batchList:        buildBatchList(core.AppData{}),
		batchCatalog:     buildArtifactList(core.AppData{}),
		batchSteps:       buildBatchSteps(core.ArtifactBatch{}, nil),
		notes:            buildNotesArea(),
		rawInput:         buildRawSQLInput(),
		rawResult:        buildRawResultView(),
		spinner:          buildSpinner(),
		connectionFocus:  focusConnectionList,
		artifactFocus:    focusPicker,
		batchFocus:       focusBatchList,
		rawFocus:         focusRawEditor,
	}

	m.setConnectionInputFocus(0)
	m.setProjectFocus(focusConnectionList)
	m.setArtifactFocus(focusPicker)
	_ = m.projectNameInput.Focus()
	return m
}

func clearStatusAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(_ time.Time) tea.Msg {
		return clearStatusMsg{}
	})
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.picker.Init(), m.startMenu.StartSpinner())
}

func (m model) withStatus(status string) model {
	m.status = status
	return m
}

func (m *model) applyData(data core.AppData) {
	m.connectionList = buildConnectionList(data)
	m.connectionInputs = buildConnectionInputs(core.DbConnection{})
	m.editingConnection = 0
	m.connectionInputIdx = 0
	m.picker = buildFilePicker()
	m.jsonPicker = buildJSONPicker()
	m.artifacts = buildArtifactList(data)
	m.batches = cloneTUIBatches(data.Batches)
	m.batchList = buildBatchList(data)
	m.batchCatalog = buildArtifactList(data)
	m.batchSteps = buildBatchSteps(core.ArtifactBatch{}, data.Artifacts)
	m.notes = buildNotesArea()
	m.rawInput = buildRawSQLInput()
	m.rawResult = buildRawResultView()
	m.rawFocus = focusRawEditor
	m.annotationOpen = false
	m.projectNameModal = false
	m.jsonPickerOpen = false
	m.projectLoaded = true
	m.setProjectFocus(focusConnectionList)
	m.setArtifactFocus(focusPicker)
	m.setBatchFocus(focusBatchList)
	if len(data.Connections) > 0 {
		m.connectionList.Select(0)
		m.loadSelectedConnectionIntoForm()
	} else {
		m.setConnectionInputFocus(0)
	}
	m.initActiveConnection()
	if len(m.batches) > 0 {
		m.batchList.Select(0)
		m.loadBatchStepsFromSelection()
	}
	m.resizeViews()
	m.loadAnnotationFromSelection()
}

func (m *model) syncCurrentScreenToCore() {
	m.syncConnectionsToCore()
	m.syncArtifactsToCore()
	m.syncBatchesToCore()
}

func (m *model) resizeViews() {
	if m.width <= 0 {
		return
	}

	if m.errorOpen {
		w, h := m.errorModalSize()
		m.errorView.SetWidth(w)
		m.errorView.SetHeight(h)
		m.errorView.SetContent(wrapText(m.errorBody, w))
	}

	startWidth := max(72, m.width-8)
	startHeight := max(8, m.height-18)
	m.startMenu.SetSize(startWidth, startHeight)
	m.jsonPicker.SetHeight(max(10, m.height-18))

	projectLeftWidth := max(34, m.width/3)
	projectRightWidth := max(54, m.width-projectLeftWidth-10)
	projectHeight := max(8, m.height-22)
	m.connectionList.SetSize(projectLeftWidth, projectHeight)
	for i := range m.connectionInputs {
		m.connectionInputs[i].SetWidth(projectRightWidth - 8)
	}

	pickerWidth := max(30, m.width/4)
	rightWidth := max(62, m.width-pickerWidth-10)
	listHeight := max(8, m.height-24)

	m.picker.SetHeight(max(8, m.height-24))
	m.artifacts.SetSize(rightWidth, listHeight)
	m.notes.SetWidth(max(52, m.width-24))
	m.notes.SetHeight(max(8, min(14, m.height-16)))

	batchColumnWidth, batchHeight := m.batchPanelSize()
	// Inner list size leaves room for the panel border/padding and the
	// title + help lines so the three boxes line up uniformly.
	batchListWidth := max(20, batchColumnWidth-6)
	batchListHeight := max(4, batchHeight-7)
	m.batchList.SetSize(batchListWidth, batchListHeight)
	m.batchCatalog.SetSize(batchListWidth, batchListHeight)
	m.batchSteps.SetSize(batchListWidth, batchListHeight)

	rawWidth := max(40, m.width-12)
	rawEditorHeight := max(6, (m.height-20)/2)
	rawResultHeight := max(6, m.height-20-rawEditorHeight)
	m.rawInput.SetWidth(rawWidth)
	m.rawInput.SetHeight(rawEditorHeight)
	m.rawResult.SetWidth(rawWidth)
	m.rawResult.SetHeight(rawResultHeight)
}

// batchPanelSize returns the uniform width and height (border included) for
// each of the three batch panels so they tile the screen evenly.
func (m model) batchPanelSize() (width, height int) {
	width = max(28, (m.width-12)/3)
	height = max(12, m.height-12)
	return width, height
}

func (m *model) setProjectFocus(focus projectPanelFocus) {
	m.connectionFocus = focus
	if focus == focusConnectionForm {
		m.setConnectionInputFocus(m.connectionInputIdx)
		return
	}
	for i := range m.connectionInputs {
		m.connectionInputs[i].Blur()
	}
}

func (m *model) setConnectionInputFocus(next int) {
	if len(m.connectionInputs) == 0 {
		return
	}

	if next < 0 {
		next = 0
	}
	if next >= len(m.connectionInputs) {
		next = len(m.connectionInputs) - 1
	}

	m.connectionInputIdx = next
	for i := range m.connectionInputs {
		if i == next && m.connectionFocus == focusConnectionForm {
			_ = m.connectionInputs[i].Focus()
			continue
		}
		m.connectionInputs[i].Blur()
	}
}

func (m *model) setArtifactFocus(focus artifactPanelFocus) {
	m.artifactFocus = focus
	m.notes.Blur()
}

func (m *model) setBatchFocus(focus batchPanelFocus) {
	m.batchFocus = focus
}

func (m *model) setRawFocus(focus rawSQLFocus) {
	m.rawFocus = focus
	if focus == focusRawEditor {
		_ = m.rawInput.Focus()
	} else {
		m.rawInput.Blur()
	}
}

// cycleRawConnection moves the raw SQL editor to the next available connection.
func (m *model) cycleRawConnection() bool {
	ids := m.connectionIDs()
	if len(ids) == 0 {
		return false
	}
	next := ids[0]
	for idx, id := range ids {
		if id == m.rawConnID {
			next = ids[(idx+1)%len(ids)]
			break
		}
	}
	m.rawConnID = next
	return true
}

func (m model) runRawSQLCmd() tea.Cmd {
	connID := m.rawConnID
	sql := m.rawInput.Value()
	return func() tea.Msg {
		output, err := m.core.ExecRaw(context.Background(), connID, sql)
		return rawSQLResultMsg{output: output, err: err}
	}
}

func (m *model) toggleArtifactFocus() {
	if m.artifactFocus == focusPicker {
		m.setArtifactFocus(focusList)
	} else {
		m.setArtifactFocus(focusPicker)
	}
}

func (m model) hasConnectionDraft() bool {
	for _, input := range m.connectionInputs {
		if strings.TrimSpace(input.Value()) != "" {
			return true
		}
	}
	return false
}

func (m *model) saveConnectionDraftIfNeeded() (string, bool) {
	if !m.hasConnectionDraft() {
		return "", true
	}
	return m.saveConnectionFromForm()
}

func (m *model) maybeSaveConnectionDraft() string {
	if !m.hasConnectionDraft() {
		return ""
	}
	if status, ok := m.saveConnectionDraftIfNeeded(); ok {
		return status
	}
	return "Catalogo SQL abierto sin guardar la conexion en edicion."
}

func (m *model) goToArtifacts() (string, bool) {
	status := m.maybeSaveConnectionDraft()
	m.screen = screenArtifacts
	if len(m.artifacts.Items()) > 0 {
		m.ensureArtifactSelection()
		m.setArtifactFocus(focusList)
	} else {
		m.setArtifactFocus(focusPicker)
	}
	m.loadAnnotationFromSelection()
	if status == "" {
		status = "Catalogo SQL listo."
	}
	return status, true
}

func (m *model) selectScreen(target screen) (string, bool) {
	if !m.projectLoaded && target != screenStart {
		return "Primero crea o carga un proyecto.", false
	}
	switch target {
	case screenStart:
		m.screen = screenStart
		m.closeAnnotationModal()
		return "", true
	case screenProject:
		m.screen = screenProject
		m.closeAnnotationModal()
		if len(m.connectionList.Items()) == 0 {
			m.clearConnectionForm()
		} else {
			if m.connectionList.GlobalIndex() < 0 {
				m.connectionList.Select(0)
			}
			m.setProjectFocus(focusConnectionList)
		}
		return "", true
	case screenArtifacts:
		m.closeAnnotationModal()
		return m.goToArtifacts()
	case screenBatches:
		m.closeAnnotationModal()
		m.maybeSaveConnectionDraft()
		m.syncArtifactsToCore()
		m.screen = screenBatches
		m.initActiveConnection()
		if len(m.batches) == 0 && len(m.artifacts.Items()) > 0 {
			m.createBatchFromArtifacts()
		}
		if len(m.batches) > 0 {
			m.ensureBatchSelection()
			m.loadBatchStepsFromSelection()
			m.setBatchFocus(focusBatchList)
		}
		return "", true
	case screenRawSQL:
		m.closeAnnotationModal()
		m.maybeSaveConnectionDraft()
		m.syncConnectionsToCore()
		m.screen = screenRawSQL
		m.initActiveConnection()
		if m.rawConnID == 0 || m.connectionNameByID(m.rawConnID) == "" {
			m.rawConnID = m.activeConnectionID
		}
		m.setRawFocus(focusRawEditor)
		return "", true
	case screenSummary:
		m.closeAnnotationModal()
		m.maybeSaveConnectionDraft()
		m.syncArtifactsToCore()
		m.syncBatchesToCore()
		m.screen = screenSummary
		return "", true
	default:
		return "", false
	}
}

func (m *model) openProjectNameModal() {
	m.projectNameModal = true
	m.jsonPickerOpen = false
	m.projectNameInput = buildProjectNameInput()
	_ = m.projectNameInput.Focus()
}

func (m *model) openJSONPicker() {
	m.jsonPickerOpen = true
	m.projectNameModal = false
	m.jsonPicker = buildJSONPicker()
}

func (m *model) closeStartModals() {
	m.projectNameModal = false
	m.jsonPickerOpen = false
}

func sanitizeProjectFilename(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, "\\", "-")
	if !strings.HasSuffix(strings.ToLower(name), defaultProjectExt) {
		name += defaultProjectExt
	}
	return name
}

func (m model) createProjectCmd(name string) tea.Cmd {
	return func() tea.Msg {
		filename := sanitizeProjectFilename(name)
		wd, err := os.Getwd()
		if err != nil {
			return projectCreateResultMsg{err: err}
		}
		path := filepath.Join(wd, filename)
		if err := m.core.Save(path); err != nil {
			return projectCreateResultMsg{err: err}
		}
		return projectCreateResultMsg{path: path}
	}
}

func (m *model) ensureArtifactSelection() {
	if len(m.artifacts.Items()) == 0 {
		return
	}
	index := m.artifacts.GlobalIndex()
	if index < 0 || index >= len(m.artifacts.Items()) {
		m.artifacts.Select(0)
	}
}

func (m *model) ensureBatchSelection() {
	if len(m.batches) == 0 {
		return
	}
	index := m.batchList.GlobalIndex()
	if index < 0 || index >= len(m.batches) {
		m.batchList.Select(0)
	}
}

func (m *model) openAnnotationModal() bool {
	index := m.currentSelectedIndex()
	if index == -1 {
		return false
	}
	m.loadAnnotationFromSelection()
	m.annotationOpen = true
	_ = m.notes.Focus()
	return true
}

func (m *model) closeAnnotationModal() {
	m.annotationOpen = false
	m.notes.Blur()
}

func (m *model) clearConnectionForm() {
	m.connectionInputs = buildConnectionInputs(core.DbConnection{})
	m.editingConnection = 0
	m.setProjectFocus(focusConnectionForm)
	m.setConnectionInputFocus(0)
	m.resizeViews()
}

func (m *model) loadSelectedConnectionIntoForm() {
	index := m.connectionList.GlobalIndex()
	if index < 0 || index >= len(m.connectionList.Items()) {
		return
	}

	item, ok := m.connectionList.Items()[index].(connectionListItem)
	if !ok {
		return
	}

	m.connectionInputs = buildConnectionInputs(core.DbConnection{
		Id:       item.ID,
		Name:     item.Name,
		Host:     item.Host,
		Port:     item.Port,
		DBName:   item.DBName,
		User:     item.User,
		Password: item.Password,
	})
	m.editingConnection = item.ID
	m.setProjectFocus(focusConnectionForm)
	m.setConnectionInputFocus(0)
	m.resizeViews()
}

func (m *model) saveConnectionFromForm() (string, bool) {
	conn := m.connectionFromInputs()
	if strings.TrimSpace(conn.Name) == "" || strings.TrimSpace(conn.Host) == "" || strings.TrimSpace(conn.DBName) == "" || strings.TrimSpace(conn.User) == "" {
		return "Name, Host, DB Name y User son obligatorios.", false
	}

	items := m.connectionList.Items()
	connections := make([]core.DbConnection, 0, len(items)+1)
	updated := false
	selectedIndex := 0

	for idx, item := range items {
		listItem, ok := item.(connectionListItem)
		if !ok {
			continue
		}

		current := core.DbConnection{
			Id:       listItem.ID,
			Name:     listItem.Name,
			Host:     listItem.Host,
			Port:     listItem.Port,
			DBName:   listItem.DBName,
			User:     listItem.User,
			Password: listItem.Password,
		}

		if listItem.ID == m.editingConnection && m.editingConnection != 0 {
			conn.Id = m.editingConnection
			current = conn
			updated = true
			selectedIndex = idx
		}

		connections = append(connections, current)
	}

	if !updated {
		conn.Id = m.nextConnectionID()
		m.editingConnection = conn.Id
		selectedIndex = len(connections)
		connections = append(connections, conn)
	}

	m.connectionList.SetItems(connectionItemsFromConnections(connections))
	m.connectionList.Select(selectedIndex)
	m.syncConnectionsToCore()

	if updated {
		return "Conexion actualizada en el proyecto.", true
	}
	return "Conexion agregada al proyecto.", true
}

func (m *model) removeSelectedConnection() bool {
	index := m.connectionList.GlobalIndex()
	if index < 0 || index >= len(m.connectionList.Items()) {
		return false
	}

	m.connectionList.RemoveItem(index)
	if len(m.connectionList.Items()) == 0 {
		m.clearConnectionForm()
		m.setProjectFocus(focusConnectionList)
	} else {
		m.connectionList.Select(min(index, len(m.connectionList.Items())-1))
		m.loadSelectedConnectionIntoForm()
	}
	m.syncConnectionsToCore()
	return true
}

func (m *model) syncConnectionsToCore() {
	items := m.connectionList.Items()
	connections := make([]core.DbConnection, 0, len(items))
	for _, item := range items {
		listItem, ok := item.(connectionListItem)
		if !ok {
			continue
		}

		connections = append(connections, core.DbConnection{
			Id:       listItem.ID,
			Name:     listItem.Name,
			Host:     listItem.Host,
			Port:     listItem.Port,
			DBName:   listItem.DBName,
			User:     listItem.User,
			Password: listItem.Password,
		})
	}
	m.core.SetConnections(connections)
	m.batches = cloneTUIBatches(m.core.Snapshot().Batches)
	m.rebuildBatchList()
}

func (m model) connectionFromInputs() core.DbConnection {
	return core.DbConnection{
		Id:       m.editingConnection,
		Name:     strings.TrimSpace(m.connectionInputs[0].Value()),
		Host:     strings.TrimSpace(m.connectionInputs[1].Value()),
		Port:     strings.TrimSpace(m.connectionInputs[2].Value()),
		DBName:   strings.TrimSpace(m.connectionInputs[3].Value()),
		User:     strings.TrimSpace(m.connectionInputs[4].Value()),
		Password: m.connectionInputs[5].Value(),
	}
}

func (m model) nextConnectionID() int {
	maxID := 0
	for _, item := range m.connectionList.Items() {
		listItem, ok := item.(connectionListItem)
		if ok && listItem.ID > maxID {
			maxID = listItem.ID
		}
	}
	return maxID + 1
}

func connectionItemFromConnection(conn core.DbConnection) connectionListItem {
	return connectionListItem{
		ID:       conn.Id,
		Name:     conn.Name,
		Host:     conn.Host,
		Port:     conn.Port,
		DBName:   conn.DBName,
		User:     conn.User,
		Password: conn.Password,
	}
}

func connectionItemsFromConnections(connections []core.DbConnection) []list.Item {
	items := make([]list.Item, 0, len(connections))
	for _, conn := range connections {
		items = append(items, connectionItemFromConnection(conn))
	}
	return items
}
