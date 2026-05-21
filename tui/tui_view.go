package tui

import (
	"fmt"
	"strings"

	figure "github.com/common-nighthawk/go-figure"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func (m model) View() tea.View {
	if m.runningLabel != "" {
		v := tea.NewView(m.renderRunningModal())
		v.AltScreen = true
		return v
	}
	if m.annotationOpen {
		v := tea.NewView(m.renderAnnotationModal())
		v.AltScreen = true
		return v
	}
	if m.projectNameModal {
		v := tea.NewView(m.renderProjectNameModal())
		v.AltScreen = true
		return v
	}
	if m.jsonPickerOpen {
		v := tea.NewView(m.renderJSONPickerModal())
		v.AltScreen = true
		return v
	}
	if m.errorOpen {
		v := tea.NewView(m.renderErrorModal())
		v.AltScreen = true
		return v
	}
	if m.batchConfirmOpen {
		v := tea.NewView(m.renderBatchConfirmModal())
		v.AltScreen = true
		return v
	}

	var content string

	switch m.screen {
	case screenStart:
		content = m.startView()
	case screenProject:
		content = m.projectView()
	case screenArtifacts:
		content = m.artifactView()
	case screenBatches:
		content = m.batchView()
	case screenRawSQL:
		content = m.rawSQLView()
	default:
		content = m.summaryView()
	}

	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m model) startView() string {
	var b strings.Builder
	b.WriteString(m.renderTabs())
	b.WriteString("\n\n")
	b.WriteString(titleStyle.Render(renderHighwayASCII()))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("Selecciona una opcion y entra directo al flujo de trabajo del proyecto."))
	b.WriteString("\n\n")
	b.WriteString(panelStyle.Width(max(72, m.width-8)).Render(m.startMenu.View()))
	b.WriteString("\n\n")
	b.WriteString(hintStyle.Render("Usa flechas para moverte. Enter confirma. Ctrl+L abre selector de JSON."))
	b.WriteString("\n")
	b.WriteString(m.renderStatus())
	return appFrameStyle.Render(b.String())
}

func (m model) projectView() string {
	var b strings.Builder
	b.WriteString(m.renderTabs())
	b.WriteString("\n\n")

	listBox := panelStyle
	formBox := panelStyle
	if m.connectionFocus == focusConnectionList {
		listBox = activePanelStyle
	} else {
		formBox = activePanelStyle
	}

	leftWidth := max(38, m.width/3)
	rightWidth := max(54, m.width-leftWidth-10)

	listTitle := sectionTitleStyle.Render("Conexiones del proyecto")
	listHelp := hintStyle.Render("Tab alterna paneles. Enter carga. x elimina. n crea nueva.")
	listBody := listTitle + "\n" + listHelp + "\n\n" + m.connectionList.View()

	modeLabel := "Nueva conexion"
	if m.editingConnection != 0 {
		modeLabel = fmt.Sprintf("Editando conexion #%d", m.editingConnection)
	}

	formTitle := sectionTitleStyle.Render(modeLabel)
	formHelp := hintStyle.Render("Tab cambia de panel. Up/Down recorre campos. Ctrl+A guarda. g avanza.")
	formFields := make([]string, 0, len(m.connectionInputs))
	for i := range m.connectionInputs {
		formFields = append(formFields, m.connectionInputs[i].View())
	}
	formBody := formTitle + "\n" + formHelp + "\n\n" + strings.Join(formFields, "\n") + "\n\n" + badgeStyle.Render("Proyecto") + " " + mutedStyle.Render("AppData agrupa todas las conexiones y scripts.")

	leftPanel := listBox.Width(leftWidth).Render(listBody)
	rightPanel := formBox.Width(rightWidth).Render(formBody)

	b.WriteString(titleStyle.Render("Proyecto / Conexiones"))
	b.WriteString("\n")
	b.WriteString("Gestiona varias conexiones dentro del mismo proyecto y alterna rapido con `Tab`.\n\n")
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel))
	b.WriteString("\n\n")
	b.WriteString(hintStyle.Render("Ctrl+Tab cambia de pestana. F1 Inicio, F2 Proyecto, F3 SQL, F4 Batch, F5 SQL Raw, F6 Resumen."))
	b.WriteString("\n")
	b.WriteString(m.renderStatus())
	return appFrameStyle.Render(b.String())
}

func (m model) artifactView() string {
	var b strings.Builder
	b.WriteString(m.renderTabs())
	b.WriteString("\n\n")

	pickerBox := panelStyle
	listBox := panelStyle
	switch m.artifactFocus {
	case focusPicker:
		pickerBox = activePanelStyle
	default:
		listBox = activePanelStyle
	}

	pickerWidth := max(30, m.width/4)
	rightWidth := max(56, m.width-pickerWidth-10)

	pickerTitle := sectionTitleStyle.Render("Filepicker .sql")
	pickerHelp := hintStyle.Render("Tab alterna paneles. Enter agrega el .sql seleccionado.")
	pickerBody := pickerTitle + "\n" + pickerHelp + "\n\n" + m.picker.View()

	listTitle := sectionTitleStyle.Render("Lista ordenable + notas")
	listHelp := hintStyle.Render("a abre anotacion. Ctrl+Up/Ctrl+Down mueve. x elimina. Enter resume.")
	listBody := listTitle + "\n" + listHelp + "\n\n" + m.artifacts.View() + "\n\n" +
		badgeStyle.Render("Anotacion") + " " + mutedStyle.Render(currentAnnotationPreview(m))

	leftPanel := pickerBox.Width(pickerWidth).Render(pickerBody)
	listPanel := listBox.Width(rightWidth).Render(listBody)

	b.WriteString(titleStyle.Render("Workspace SQL"))
	b.WriteString("\n")
	b.WriteString("Selecciona `.sql`, ordenalos y abre una anotacion modal por cada script cuando haga falta.\n\n")
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, listPanel))
	b.WriteString("\n\n")
	b.WriteString(hintStyle.Render("Tab rota entre picker y lista. Ctrl+Tab cambia de pestana. F3 vuelve aqui."))
	b.WriteString("\n")
	b.WriteString(m.renderStatus())

	return appFrameStyle.Render(b.String())
}

func (m model) batchView() string {
	var b strings.Builder
	b.WriteString(m.renderTabs())
	b.WriteString("\n\n")

	listBox := panelStyle
	catalogBox := panelStyle
	stepsBox := panelStyle
	if m.batchFocus == focusBatchList {
		listBox = activePanelStyle
	} else if m.batchFocus == focusBatchCatalog {
		catalogBox = activePanelStyle
	} else {
		stepsBox = activePanelStyle
	}

	columnWidth, columnHeight := m.batchPanelSize()

	listTitle := sectionTitleStyle.Render("Batch del proyecto")
	listHelp := hintStyle.Render("n crea batch. c cambia conexion. e ejecuta (pide confirmar). x elimina. Enter entra al catalogo.")
	listBody := listTitle + "\n" + listHelp + "\n\n" + m.batchList.View()

	catalogTitle := sectionTitleStyle.Render("Catalogo SQL")
	catalogHelp := hintStyle.Render("Enter agrega y te deja aqui para seguir agregando. Tab pasa al orden.")
	catalogBody := catalogTitle + "\n" + catalogHelp + "\n\n" + m.batchCatalog.View()

	stepsTitle := sectionTitleStyle.Render("Orden real de ejecucion")
	stepsHelp := hintStyle.Render("Espacio activa/desactiva. x quita SQL. Ctrl+Up/Down reordena. r reconstruye.")
	stepsBody := stepsTitle + "\n" + stepsHelp + "\n\n" + m.batchSteps.View()

	leftPanel := listBox.Width(columnWidth).Height(columnHeight).Render(listBody)
	middlePanel := catalogBox.Width(columnWidth).Height(columnHeight).Render(catalogBody)
	rightPanel := stepsBox.Width(columnWidth).Height(columnHeight).Render(stepsBody)

	b.WriteString(titleStyle.Render("Batch / Corridas"))
	b.WriteString("\n")
	b.WriteString(m.renderActiveConnectionBar())
	b.WriteString("\n\n")
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, middlePanel, rightPanel))
	b.WriteString("\n\n")
	b.WriteString(hintStyle.Render("Tab alterna batch/catalogo/pasos. Ctrl+Tab cambia de pestana. F4 abre batch."))
	b.WriteString("\n")
	b.WriteString(m.renderStatus())

	return appFrameStyle.Render(b.String())
}

func (m model) rawSQLView() string {
	var b strings.Builder
	b.WriteString(m.renderTabs())
	b.WriteString("\n\n")

	editorBox := panelStyle
	resultBox := panelStyle
	if m.rawFocus == focusRawEditor {
		editorBox = activePanelStyle
	} else {
		resultBox = activePanelStyle
	}

	width := max(40, m.width-12)

	editorTitle := sectionTitleStyle.Render("Consulta SQL")
	editorHelp := hintStyle.Render("Ctrl+E ejecuta. Ctrl+N cambia conexion. Tab pasa a resultados.")
	editorBody := editorTitle + "\n" + editorHelp + "\n\n" + m.rawInput.View()

	resultTitle := sectionTitleStyle.Render("Resultados")
	resultHelp := hintStyle.Render("Flechas / PgUp / PgDn desplazan. Tab vuelve al editor.")
	resultBody := resultTitle + "\n" + resultHelp + "\n\n" + m.rawResult.View()

	connLabel := m.connectionLabelByID(m.rawConnID)
	connBar := badgeStyle.Render("Conexion") + " "
	if strings.TrimSpace(connLabel) == "" {
		connBar += errStyle.Render("ninguna") + " " + hintStyle.Render("(agrega conexiones en F2)")
	} else {
		connBar += sectionTitleStyle.Render(connLabel) + "  " + hintStyle.Render("(Ctrl+N cambia)")
	}

	b.WriteString(titleStyle.Render("SQL Raw / Consola"))
	b.WriteString("\n")
	b.WriteString(connBar)
	b.WriteString("\n\n")
	b.WriteString(editorBox.Width(width).Render(editorBody))
	b.WriteString("\n")
	b.WriteString(resultBox.Width(width).Render(resultBody))
	b.WriteString("\n\n")
	b.WriteString(hintStyle.Render("Ctrl+E ejecuta contra la conexion mostrada. Ctrl+Tab cambia de pestana."))
	b.WriteString("\n")
	b.WriteString(m.renderStatus())

	return appFrameStyle.Render(b.String())
}

func (m model) renderRunningModal() string {
	body := m.spinner.View() + " " + sectionTitleStyle.Render(m.runningLabel) + "\n\n" +
		mutedStyle.Render("Espera a que termine. La interfaz esta bloqueada para evitar cambios.")
	modal := modalBoxStyle.Width(min(max(48, m.width-20), 72)).Render(body)
	return appFrameStyle.Render(strings.Join([]string{
		m.renderTabs(),
		"",
		titleStyle.Render("Ejecutando"),
		"",
		modal,
	}, "\n"))
}

func (m model) summaryView() string {
	data := m.core.Snapshot()
	lines := []string{
		m.renderTabs(),
		"",
		titleStyle.Render("Resumen del Proyecto"),
		"",
		sectionTitleStyle.Render(fmt.Sprintf("Conexiones (%d):", len(data.Connections))),
	}

	if len(data.Connections) == 0 {
		lines = append(lines, "No hay conexiones cargadas.")
	} else {
		for _, conn := range data.Connections {
			target := conn.Host
			if conn.Port != "" {
				target = target + ":" + conn.Port
			}
			lines = append(lines, fmt.Sprintf("- %s -> %s/%s (%s)", conn.Name, target, conn.DBName, conn.User))
		}
	}

	lines = append(lines, "")
	lines = append(lines, sectionTitleStyle.Render("SQL en orden actual:"))

	if len(data.Artifacts) == 0 {
		lines = append(lines, "No hay scripts cargados.")
	} else {
		for idx, artifact := range data.Artifacts {
			noteState := "sin notas"
			if strings.TrimSpace(artifact.Annotation) != "" {
				noteState = "con notas"
			}
			lines = append(lines, fmt.Sprintf("%02d. %s [%s]", idx+1, artifact.Path, noteState))
		}
	}

	lines = append(lines, "")
	lines = append(lines, sectionTitleStyle.Render(fmt.Sprintf("Batch (%d):", len(data.Batches))))
	if len(data.Batches) == 0 {
		lines = append(lines, "No hay batch cargados.")
	} else {
		for _, batch := range data.Batches {
			enabledCount := 0
			for _, step := range batch.Steps {
				if step.Enabled {
					enabledCount++
				}
			}
			lines = append(lines, fmt.Sprintf("- %s [%d/%d activos]", batch.Name, enabledCount, len(batch.Steps)))
		}
	}

	lines = append(lines, "")
	lines = append(lines, hintStyle.Render("Ctrl+Tab cambia de pestana. F6 abre este resumen."))
	lines = append(lines, m.renderStatus())

	return appFrameStyle.Render(strings.Join(lines, "\n"))
}

func (m model) renderStatus() string {
	if m.status == "" {
		return ""
	}
	if strings.HasPrefix(m.status, "Error") || strings.HasSuffix(m.status, "obligatorios.") {
		return errStyle.Render(m.status)
	}
	return okStyle.Render(m.status)
}

func currentAnnotationPreview(m model) string {
	index := m.currentSelectedIndex()
	if index == -1 {
		return "Selecciona un SQL y presiona `a` para escribir su anotacion."
	}

	item, ok := m.artifacts.Items()[index].(sqlArtifactItem)
	if !ok || strings.TrimSpace(item.Annotation) == "" {
		return "Este SQL aun no tiene anotacion. Presiona `a`."
	}

	preview := strings.ReplaceAll(strings.TrimSpace(item.Annotation), "\n", " ")
	if len(preview) > 72 {
		preview = preview[:72] + "..."
	}
	return preview
}

func (m model) renderAnnotationModal() string {
	index := m.currentSelectedIndex()
	title := "Anotacion SQL"
	if index != -1 {
		if item, ok := m.artifacts.Items()[index].(sqlArtifactItem); ok {
			title = "Anotacion: " + item.Name
		}
	}

	body := sectionTitleStyle.Render(title) + "\n" +
		hintStyle.Render("Ctrl+S o Ctrl+Enter guardan. Esc cierra.") + "\n\n" +
		m.notes.View()

	modalWidth := min(max(68, m.width-20), 96)
	modal := modalBoxStyle.Width(modalWidth).Render(body)
	lines := []string{
		m.renderTabs(),
		"",
		titleStyle.Render("Editor de Anotacion"),
		"",
		modal,
		"",
		m.renderStatus(),
	}
	return appFrameStyle.Render(strings.Join(lines, "\n"))
}

func (m model) renderTabs() string {
	tabs := []struct {
		label  string
		target screen
	}{
		{label: "F1 Inicio", target: screenStart},
		{label: "F2 Proyecto", target: screenProject},
		{label: "F3 SQL", target: screenArtifacts},
		{label: "F4 Batch", target: screenBatches},
		{label: "F5 SQL Raw", target: screenRawSQL},
		{label: "F6 Resumen", target: screenSummary},
	}

	parts := make([]string, 0, len(tabs))
	for _, tab := range tabs {
		style := tabStyle
		if !m.projectLoaded && tab.target != screenStart {
			style = tabStyle.Foreground(lipgloss.Color("#4B6077"))
		} else if m.screen == tab.target {
			style = activeTabStyle
		}
		parts = append(parts, style.Render(tab.label))
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func (m model) renderProjectNameModal() string {
	body := sectionTitleStyle.Render("Crear Proyecto") + "\n" +
		hintStyle.Render("Escribe el nombre. Enter crea el JSON. Esc cancela.") + "\n\n" +
		m.projectNameInput.View()
	return appFrameStyle.Render(strings.Join([]string{
		m.renderTabs(),
		"",
		modalBoxStyle.Width(min(max(56, m.width-20), 80)).Render(body),
		"",
		m.renderStatus(),
	}, "\n"))
}

func (m model) renderJSONPickerModal() string {
	body := sectionTitleStyle.Render("Cargar Proyecto JSON") + "\n" +
		hintStyle.Render("Flechas mueven. Enter/derecha entra a carpeta. Izquierda/Backspace vuelve. Enter sobre un .json lo carga. Esc cancela.") + "\n\n" +
		m.jsonPicker.View()
	return appFrameStyle.Render(strings.Join([]string{
		m.renderTabs(),
		"",
		modalBoxStyle.Width(min(max(72, m.width-20), 100)).Render(body),
		"",
		m.renderStatus(),
	}, "\n"))
}

func (m model) renderErrorModal() string {
	header := errStyle.Render(m.errorTitle)
	help := hintStyle.Render("Flechas / PgUp / PgDn desplazan. Esc o Enter cierra.")
	box := modalBoxStyle.Render(header + "\n" + help + "\n\n" + m.errorView.View())
	return appFrameStyle.Render(strings.Join([]string{
		titleStyle.Render("Detalle del error"),
		"",
		box,
	}, "\n"))
}

func (m model) renderActiveConnectionBar() string {
	label := m.connectionLabelByID(m.activeConnectionID)
	if strings.TrimSpace(label) == "" {
		return badgeStyle.Render("Conexion activa") + " " +
			errStyle.Render("ninguna") + " " +
			hintStyle.Render("(agrega conexiones en F2)")
	}
	return badgeStyle.Render("Conexion activa") + " " +
		sectionTitleStyle.Render(label) + "  " +
		hintStyle.Render("(c cambia para todos los batches)")
}

func (m model) renderBatchConfirmModal() string {
	index := m.batchList.GlobalIndex()
	name := "el batch seleccionado"
	connectionID := m.activeConnectionID
	enabled, total := 0, 0
	if index >= 0 && index < len(m.batches) {
		batch := m.batches[index]
		name = batch.Name
		connectionID = batch.ConnectionID
		total = len(batch.Steps)
		for _, step := range batch.Steps {
			if step.Enabled {
				enabled++
			}
		}
	}

	dangerous := len(m.batchDangers) > 0
	remote := m.batchConfirmRemote
	risky := dangerous || remote
	panelHeight := max(8, 4+len(m.batchDangers))

	// Connection panel: deliberately large and highlighted so the target DB is
	// impossible to miss before running anything destructive.
	connLabel := m.connectionLabelByID(connectionID)
	if strings.TrimSpace(connLabel) == "" {
		connLabel = "ninguna conexion asignada"
	}
	connHeader := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#050816")).
		Background(lipgloss.Color("#FFD166")).Padding(0, 1).Render(" CONEXION DESTINO ")
	connName := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFF3B0")).
		Render(connLabel)
	connBody := connHeader + "\n\n" + connName
	if remote {
		connBody += "\n\n" + errStyle.Bold(true).Render("⚠ CONEXION REMOTA (no es localhost)")
	} else {
		connBody += "\n\n" + okStyle.Render("✓ localhost")
	}
	connBorder := lipgloss.Color("#FFD166")
	if risky {
		connBorder = lipgloss.Color("#FF5C5C")
	}
	connPanel := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder()).
		BorderForeground(connBorder).
		Padding(1, 2).
		Width(max(34, (min(max(72, m.width-20), 110)-10)/2)).
		Height(panelHeight).
		Render(connBody)

	// Danger panel: the result of scanning the SQL for DROP/DELETE/TRUNCATE.
	var dangerLines []string
	dangerBorder := lipgloss.Color("#52FFA8")
	if dangerous {
		dangerBorder = lipgloss.Color("#FF5C5C")
		dangerLines = append(dangerLines, errStyle.Bold(true).Render("⚠ OPERACIONES DESTRUCTIVAS"))
		dangerLines = append(dangerLines, "")
		for _, d := range m.batchDangers {
			dangerLines = append(dangerLines, errStyle.Render(fmt.Sprintf("  %2d x %s", d.Count, d.Label)))
		}
	} else {
		dangerLines = append(dangerLines, okStyle.Bold(true).Render("✓ Sin DROP / DELETE / TRUNCATE"))
		dangerLines = append(dangerLines, "")
		dangerLines = append(dangerLines, mutedStyle.Render("El escaneo no encontro\noperaciones destructivas."))
	}
	dangerPanel := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder()).
		BorderForeground(dangerBorder).
		Padding(1, 2).
		Width(max(34, (min(max(72, m.width-20), 110)-10)/2)).
		Height(panelHeight).
		Render(strings.Join(dangerLines, "\n"))

	panels := lipgloss.JoinHorizontal(lipgloss.Top, connPanel, "  ", dangerPanel)

	var prompt string
	if risky {
		reason := "Escenario peligroso."
		switch {
		case dangerous && remote:
			reason = "Operaciones destructivas sobre una base REMOTA."
		case remote:
			reason = "Vas a ejecutar contra una base REMOTA (no localhost)."
		}
		field := lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#FF5C5C")).
			Padding(0, 1).
			Render(m.batchConfirmInput + "▏")
		prompt = errStyle.Bold(true).Render(reason) + " " +
			"Escribe exactamente  " + okStyle.Bold(true).Render(m.batchConfirmPhrase) + "  y Enter para ejecutar:\n" +
			field + "\n" +
			hintStyle.Render("Esc cancela.")
	} else {
		prompt = "Esto correra los SQL activos contra la conexion mostrada.\n\n" +
			hintStyle.Render("y / Enter ejecuta. n / Esc cancela.")
	}

	batchBanner := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#050816")).
		Background(lipgloss.Color("#78F7FF")).
		Padding(0, 2).
		Render("▶ BATCH A EJECUTAR:  " + name)
	stepInfo := sectionTitleStyle.Render(fmt.Sprintf("%d de %d pasos activos", enabled, total))
	body := batchBanner + "\n" + stepInfo + "\n\n" + panels + "\n\n" + prompt

	return appFrameStyle.Render(strings.Join([]string{
		m.renderTabs(),
		"",
		titleStyle.Render("Confirmar ejecucion"),
		"",
		modalBoxStyle.Width(min(max(72, m.width-20), 110)).Render(body),
		"",
		m.renderStatus(),
	}, "\n"))
}

// wrapText hard-wraps s to the given width so long single-line messages (like
// database errors) stay fully visible inside a viewport instead of running off
// the right edge.
func wrapText(s string, width int) string {
	if width <= 0 {
		return s
	}
	return lipgloss.NewStyle().Width(width).Render(s)
}

func renderHighwayASCII() string {
	lines := figure.NewFigure("HIGHWAY", "standard", true).Slicify()
	return strings.Join(lines, "\n")
}
