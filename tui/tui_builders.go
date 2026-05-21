package tui

import (
	"os"
	"path/filepath"
	"strings"

	"highway/core"

	"charm.land/bubbles/v2/filepicker"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
)

func buildStartMenu() list.Model {
	items := []list.Item{
		startOption{
			title: "Cargar desde JSON",
			desc:  "Lee `app-data.json` y rellena el proyecto, conexiones, lista SQL y anotaciones guardadas.",
		},
		startOption{
			title: "Crear nuevo proyecto",
			desc:  "Inicia una sesion vacia para registrar varias conexiones y nuevos scripts SQL.",
		},
	}

	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true

	l := list.New(items, delegate, 72, 8)
	l.Title = "Inicio"
	l.SetShowFilter(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()
	return l
}

func buildConnectionInputs(conn core.DbConnection) []textinput.Model {
	fields := []struct {
		prompt      string
		placeholder string
		value       string
		password    bool
	}{
		{prompt: "Name: ", placeholder: "reporting-prod", value: conn.Name},
		{prompt: "Host: ", placeholder: "localhost", value: conn.Host},
		{prompt: "Port: ", placeholder: "5432", value: conn.Port},
		{prompt: "DB Name: ", placeholder: "my_database", value: conn.DBName},
		{prompt: "User: ", placeholder: "postgres", value: conn.User},
		{prompt: "Password: ", placeholder: "secret", value: conn.Password, password: true},
	}

	inputs := make([]textinput.Model, len(fields))
	for i, field := range fields {
		input := textinput.New()
		input.Prompt = field.prompt
		input.Placeholder = field.placeholder
		input.SetValue(field.value)
		input.SetWidth(44)
		if field.password {
			input.EchoMode = textinput.EchoPassword
		}
		inputs[i] = input
	}
	return inputs
}

func buildConnectionList(data core.AppData) list.Model {
	items := make([]list.Item, 0, len(data.Connections))
	for _, conn := range data.Connections {
		items = append(items, connectionItemFromConnection(conn))
	}

	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true

	l := list.New(items, delegate, 42, 12)
	l.Title = "Conexiones"
	l.SetShowFilter(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()
	return l
}

func buildFilePicker() filepicker.Model {
	fp := filepicker.New()
	fp.AllowedTypes = []string{".sql"}
	wd, err := os.Getwd()
	if err != nil {
		wd = "."
	}
	fp.CurrentDirectory = wd
	fp.FileAllowed = true
	fp.DirAllowed = true
	fp.ShowHidden = false
	fp.SetHeight(12)
	return fp
}

func buildJSONPicker() filepicker.Model {
	fp := filepicker.New()
	fp.AllowedTypes = []string{defaultProjectExt}
	wd, err := os.Getwd()
	if err != nil {
		wd = "."
	}
	fp.CurrentDirectory = wd
	fp.FileAllowed = true
	fp.DirAllowed = true
	fp.ShowHidden = false
	fp.SetHeight(16)
	return fp
}

func buildProjectNameInput() textinput.Model {
	input := textinput.New()
	input.Prompt = "Nombre del proyecto: "
	input.Placeholder = "mi-proyecto"
	input.SetWidth(40)
	return input
}

func buildArtifactList(data core.AppData) list.Model {
	items := make([]list.Item, 0, len(data.Artifacts))
	for idx, artifact := range data.Artifacts {
		name := artifact.Name
		if strings.TrimSpace(name) == "" {
			name = filepath.Base(artifact.Path)
		}
		items = append(items, sqlArtifactItem{
			ID:         artifact.ID,
			Order:      idx + 1,
			Name:       name,
			Path:       artifact.Path,
			Annotation: artifact.Annotation,
		})
	}

	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true

	l := list.New(items, delegate, 48, 10)
	l.Title = "Scripts SQL"
	l.SetShowFilter(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()
	return l
}

func buildBatchList(data core.AppData) list.Model {
	items := make([]list.Item, 0, len(data.Batches))
	connectionNames := make(map[int]string, len(data.Connections))
	for _, conn := range data.Connections {
		connectionNames[conn.Id] = conn.Name
	}

	for _, batch := range data.Batches {
		enabledCount := 0
		for _, step := range batch.Steps {
			if step.Enabled {
				enabledCount++
			}
		}
		items = append(items, batchListItem{
			ID:             batch.ID,
			Name:           batch.Name,
			Details:        batch.Description,
			ConnectionName: connectionNames[batch.ConnectionID],
			EnabledCount:   enabledCount,
			StepCount:      len(batch.Steps),
		})
	}

	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true

	l := list.New(items, delegate, 36, 12)
	l.Title = "Batch"
	l.SetShowFilter(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()
	return l
}

func buildBatchSteps(batch core.ArtifactBatch, artifacts []core.Artifact) list.Model {
	artifactByID := make(map[int]core.Artifact, len(artifacts))
	for _, artifact := range artifacts {
		artifactByID[artifact.ID] = artifact
	}

	items := make([]list.Item, 0, len(batch.Steps))
	for idx, step := range batch.Steps {
		artifact, ok := artifactByID[step.ArtifactID]
		if !ok {
			continue
		}
		items = append(items, batchStepItem{
			ArtifactID:   step.ArtifactID,
			Order:        idx + 1,
			Enabled:      step.Enabled,
			ArtifactName: artifact.Name,
			Path:         artifact.Path,
			Annotation:   artifact.Annotation,
		})
	}

	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false

	l := list.New(items, delegate, 54, 12)
	l.Title = "Pasos del Batch"
	l.SetShowFilter(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()
	return l
}

func buildRawSQLInput() textarea.Model {
	ta := textarea.New()
	ta.Placeholder = "Escribe SQL para ejecutar directo contra la conexion activa.\nEj: SELECT * FROM usuarios LIMIT 50;"
	ta.Prompt = "│ "
	ta.ShowLineNumbers = true
	ta.SetWidth(72)
	ta.SetHeight(10)
	return ta
}

func buildRawResultView() viewport.Model {
	vp := viewport.New()
	vp.SetWidth(72)
	vp.SetHeight(10)
	vp.SetContent(mutedStyle.Render("Aun no has ejecutado ninguna consulta. Escribe SQL y presiona Ctrl+E."))
	return vp
}

func buildSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#78F7FF"))
	return s
}

func buildNotesArea() textarea.Model {
	ta := textarea.New()
	ta.Placeholder = "Escribe la anotacion del SQL.\nEj: orden de ejecucion, dependencias, validaciones, rollback..."
	ta.Prompt = ""
	ta.ShowLineNumbers = false
	ta.SetWidth(72)
	ta.SetHeight(12)
	return ta
}
