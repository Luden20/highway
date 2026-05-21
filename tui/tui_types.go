package tui

import (
	"fmt"
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

const defaultProjectExt = ".json"

type screen int

const (
	screenStart screen = iota
	screenProject
	screenArtifacts
	screenBatches
	screenRawSQL
	screenSummary
)

type rawSQLFocus int

const (
	focusRawEditor rawSQLFocus = iota
	focusRawResult
)

type projectPanelFocus int

const (
	focusConnectionList projectPanelFocus = iota
	focusConnectionForm
)

type artifactPanelFocus int

const (
	focusPicker artifactPanelFocus = iota
	focusList
)

type batchPanelFocus int

const (
	focusBatchList batchPanelFocus = iota
	focusBatchCatalog
	focusBatchSteps
)

type clearStatusMsg struct{}

type batchRunResultMsg struct {
	err error
}

type rawSQLResultMsg struct {
	output string
	err    error
}

type projectCreateResultMsg struct {
	err  error
	path string
}

type startOption struct {
	title string
	desc  string
}

func (o startOption) FilterValue() string { return o.title }
func (o startOption) Title() string       { return o.title }
func (o startOption) Description() string { return o.desc }

type connectionListItem struct {
	ID       int
	Name     string
	Host     string
	Port     string
	DBName   string
	User     string
	Password string
}

func (c connectionListItem) FilterValue() string {
	return c.Name + " " + c.Host + " " + c.DBName + " " + c.User
}

func (c connectionListItem) Title() string {
	return fmt.Sprintf("%s (#%d)", c.Name, c.ID)
}

func (c connectionListItem) Description() string {
	if c.Port == "" {
		return fmt.Sprintf("%s@%s/%s", c.User, c.Host, c.DBName)
	}
	return fmt.Sprintf("%s@%s:%s/%s", c.User, c.Host, c.Port, c.DBName)
}

type sqlArtifactItem struct {
	ID         int
	Order      int
	Name       string
	Path       string
	Annotation string
}

func (a sqlArtifactItem) FilterValue() string { return a.Name + " " + a.Path + " " + a.Annotation }
func (a sqlArtifactItem) Title() string {
	return fmt.Sprintf("%02d. %s", a.Order, a.Name)
}
func (a sqlArtifactItem) Description() string {
	note := "sin anotacion"
	if trimmed := strings.TrimSpace(a.Annotation); trimmed != "" {
		note = trimmed
	}
	note = strings.ReplaceAll(note, "\n", " ")
	if len(note) > 48 {
		note = note[:48] + "..."
	}
	return note
}

type batchListItem struct {
	ID             int
	Name           string
	Details        string
	ConnectionName string
	EnabledCount   int
	StepCount      int
}

func (b batchListItem) FilterValue() string {
	return b.Name + " " + b.Details + " " + b.ConnectionName
}

func (b batchListItem) Title() string {
	return b.Name
}

func (b batchListItem) Description() string {
	return fmt.Sprintf("%d/%d activos | %s", b.EnabledCount, b.StepCount, b.Details)
}

type batchStepItem struct {
	ArtifactID   int
	Order        int
	Enabled      bool
	ArtifactName string
	Path         string
	Annotation   string
}

func (b batchStepItem) FilterValue() string {
	return b.ArtifactName + " " + b.Path + " " + b.Annotation
}

func (b batchStepItem) Title() string {
	state := "OFF"
	if b.Enabled {
		state = "ON"
	}
	return fmt.Sprintf("%02d. [%s] %s", b.Order, state, b.ArtifactName)
}

func (b batchStepItem) Description() string {
	note := strings.TrimSpace(strings.ReplaceAll(b.Annotation, "\n", " "))
	if note == "" {
		note = "sin anotacion"
	}
	if len(note) > 56 {
		note = note[:56] + "..."
	}
	return note
}

type model struct {
	core               *core.AppCore
	screen             screen
	startMenu          list.Model
	jsonPicker         filepicker.Model
	projectNameInput   textinput.Model
	projectLoaded      bool
	projectPath        string
	projectNameModal   bool
	jsonPickerOpen     bool
	connectionList     list.Model
	connectionInputs   []textinput.Model
	connectionFocus    projectPanelFocus
	connectionInputIdx int
	editingConnection  int
	picker             filepicker.Model
	artifacts          list.Model
	batches            []core.ArtifactBatch
	batchList          list.Model
	batchCatalog       list.Model
	batchSteps         list.Model
	batchFocus         batchPanelFocus
	activeConnectionID int
	notes              textarea.Model
	artifactFocus      artifactPanelFocus
	annotationOpen     bool
	batchConfirmOpen   bool
	batchDangers       []core.DangerStat
	batchConfirmInput  string
	batchConfirmRemote bool
	batchConfirmPhrase string
	errorOpen          bool
	errorTitle         string
	errorBody          string
	errorView          viewport.Model
	rawInput           textarea.Model
	rawResult          viewport.Model
	rawFocus           rawSQLFocus
	rawConnID          int
	spinner            spinner.Model
	runningLabel       string
	status             string
	width              int
	height             int
}

var (
	appFrameStyle = lipgloss.NewStyle().
			Padding(1, 2).
			Foreground(lipgloss.Color("#D9F7FF"))

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#78F7FF"))

	hintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7BA8C4"))

	okStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#52FFA8"))

	errStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6BD6"))

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#1B355A")).
			Padding(1, 2)

	activePanelStyle = panelStyle.
				BorderForeground(lipgloss.Color("#78F7FF"))

	sectionTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#B5FF5E"))

	mutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#96A9C7"))

	badgeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#050816")).
			Background(lipgloss.Color("#B5FF5E")).
			Padding(0, 1).
			Bold(true)

	modalOverlayStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#D9F7FF"))

	modalBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("#FF6BD6")).
			Padding(1, 2)

	tabStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(lipgloss.Color("#1B355A")).
			Foreground(lipgloss.Color("#7BA8C4"))

	activeTabStyle = tabStyle.
			Bold(true).
			Foreground(lipgloss.Color("#78F7FF")).
			BorderForeground(lipgloss.Color("#78F7FF"))
)
