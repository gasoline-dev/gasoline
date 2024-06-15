package cmd

import (
	"errors"
	"fmt"
	"gas/helpers"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var itCmd = &cobra.Command{
	Use:   "it",
	Short: "Create project",
	Run: func(cmd *cobra.Command, args []string) {
		p := tea.NewProgram(initialModel(), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			fmt.Printf("Error: %v", err)
			os.Exit(1)
		}
	},
}

type model struct {
	state                                   state
	dirPathInput                            textinput.Model
	confirmEmptyDirPathInput                textinput.Model
	emptyingDirPathViewLoaded               bool
	spinner                                 spinner.Model
	selectPackageManager                    selectModel
	packageManager                          string
	downloadingNewProjectTemplateViewLoaded bool
}

type state string

const (
	_dirPathInput                  state = "_dirPathInput"
	_confirmEmptyDirPathInput      state = "_confirmEmptyDirPathInput"
	_emptyingDirPath               state = "_emptyingDirPath"
	_selectingPackageManager       state = "_selectingPackageManager"
	_downloadingNewProjectTemplate state = "_downloadingNewProjectTemplate"
)

func initialModel() model {
	dirPathInput := textinput.New()
	dirPathInput.Placeholder = "./example"
	dirPathInput.Focus()

	confirmEmptyDirPathInput := textinput.New()
	confirmEmptyDirPathInput.CharLimit = 1

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("202"))

	selectPackageManager := NewSelect()
	selectPackageManager.choices = []string{"npm", "pnpm"}
	selectPackageManager.choice = selectPackageManager.choices[selectPackageManager.cursor]

	return model{
		state:                                   _dirPathInput,
		dirPathInput:                            dirPathInput,
		confirmEmptyDirPathInput:                confirmEmptyDirPathInput,
		emptyingDirPathViewLoaded:               false,
		spinner:                                 s,
		selectPackageManager:                    selectPackageManager,
		packageManager:                          "",
		downloadingNewProjectTemplateViewLoaded: false,
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) View() string {
	switch m.state {
	case _dirPathInput:
		return dirPathInputView(m)
	case _confirmEmptyDirPathInput:
		return confirmEmptyDirPathInputView(m)
	case _emptyingDirPath:
		return emptyingDirView(m)
	case _selectingPackageManager:
		return selectingPackageManagerView(m)
	case _downloadingNewProjectTemplate:
		return downloadingNewProjectTemplateView(m)
	default:
		return ""
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		k := msg.String()
		if k == "esc" || k == "q" {
			return m, tea.Quit
		}
	}

	switch m.state {
	case _dirPathInput:
		return dirPathInputUpdate(m, msg)
	case _confirmEmptyDirPathInput:
		return confirmEmptyDirPathUpdate(m, msg)
	case _emptyingDirPath:
		return emptyingDirUpdate(m, msg)
	case _selectingPackageManager:
		return selectingPackageManagerUpdate(m, msg)
	case _downloadingNewProjectTemplate:
		return downloadingNewProjectTemplateUpdate(m, msg)
	default:
		return m, nil
	}
}

func dirPathInputView(m model) string {
	s := fmt.Sprintf(
		"Directory path:\n\n%s\n\n",
		m.dirPathInput.View())

	if m.dirPathInput.Err != nil {
		var inputErr *InputErr
		switch {
		case errors.As(m.dirPathInput.Err, &inputErr):
			s += fmt.Sprintf("%v\n\n", m.dirPathInput.Err)
		default:
			s += fmt.Sprintf("Error: %v\n\n", m.dirPathInput.Err)
		}
	}

	s += helpView()

	return s
}

func dirPathInputUpdate(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			if m.dirPathInput.Value() == "" {
				m.dirPathInput.Err = &InputErr{
					Msg: "Directory path is required",
				}
				return m, nil
			}

			dirPathInputExists, err := helpers.CheckIfDirExists(m.dirPathInput.Value())
			if err != nil {
				m.dirPathInput.Err = err
				return m, nil
			}

			if dirPathInputExists {
				dirPathEntries, err := os.ReadDir("./")
				if err != nil {
					m.dirPathInput.Err = err
					return m, nil
				}

				if len(dirPathEntries) > 0 {
					m.state = _confirmEmptyDirPathInput
					m.dirPathInput.Blur()
					return m, m.confirmEmptyDirPathInput.Focus()
				}
			}

			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.dirPathInput, cmd = m.dirPathInput.Update(msg)
	return m, cmd
}

func helpView() string {
	return "Press esc to exit\n"
}

func confirmEmptyDirPathInputView(m model) string {
	resolvedPath, _ := filepath.Abs(m.dirPathInput.Value())
	s := fmt.Sprintf("%s is not empty.\n\nEmpty it? (y/n)\n\n%s\n\n", resolvedPath, m.confirmEmptyDirPathInput.View())
	s += helpView()
	return s
}

func confirmEmptyDirPathUpdate(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "enter":
			lowercaseValue := strings.ToLower(m.confirmEmptyDirPathInput.Value())
			if lowercaseValue == "y" {
				m.state = _emptyingDirPath
				return m, nil
			}
			return m, nil
		}
	}

	var cmd tea.Cmd

	m.confirmEmptyDirPathInput, cmd = m.confirmEmptyDirPathInput.Update(msg)

	return m, cmd
}

type emptyingDirPathDone bool

func emptyDir() tea.Msg {
	time.Sleep(250 * time.Millisecond)
	return emptyingDirPathDone(true)
}

func emptyingDirView(m model) string {
	return fmt.Sprintf("%s Emptying dir view", m.spinner.View())
}

func emptyingDirUpdate(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	if !m.emptyingDirPathViewLoaded {
		m.emptyingDirPathViewLoaded = true
		return m, tea.Batch(m.spinner.Tick, emptyDir)
	}

	switch msg := msg.(type) {
	case emptyingDirPathDone:
		m.state = _selectingPackageManager
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

type updateNextStateEvent bool

func updateNextState() tea.Msg {
	return updateNextStateEvent(true)
}

func selectingPackageManagerView(m model) string {
	s := "Select package manager:\n\n"
	s += m.selectPackageManager.View()
	s += "\n\n"
	s += helpView()
	return s
}

func selectingPackageManagerUpdate(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			m.state = _downloadingNewProjectTemplate
			m.packageManager = m.selectPackageManager.choice
			return m, updateNextState
		}
	}

	var cmd tea.Cmd
	m.selectPackageManager, cmd = m.selectPackageManager.Update(msg)
	return m, cmd
}

type selectModel struct {
	cursor  int
	choice  string
	choices []string
}

func NewSelect() selectModel {
	return selectModel{}
}

func (m selectModel) Init() tea.Cmd {
	return nil
}

func (m selectModel) View() string {
	s := strings.Builder{}

	for i := 0; i < len(m.choices); i++ {
		if m.cursor == i {
			s.WriteString("(•) ")
		} else {
			s.WriteString("( ) ")
		}
		s.WriteString(m.choices[i])
		s.WriteString("\n")
	}

	return s.String()
}

func (m selectModel) Update(msg tea.Msg) (selectModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "down", "j":
			m.cursor++
			if m.cursor >= len(m.choices) {
				m.cursor = 0
			}
			m.choice = m.choices[m.cursor]

		case "up", "k":
			m.cursor--
			if m.cursor < 0 {
				m.cursor = len(m.choices) - 1
			}
			m.choice = m.choices[m.cursor]

		case "tab":
			if m.cursor == len(m.choices)-1 {
				m.cursor = 0
			} else {
				m.cursor++
			}
			m.choice = m.choices[m.cursor]
		}
	}

	return m, nil
}

type InputErr struct {
	Msg string
}

func (e *InputErr) Error() string {
	return e.Msg
}

func downloadingNewProjectTemplateView(m model) string {
	resolvedPath, _ := filepath.Abs(m.dirPathInput.Value())
	return fmt.Sprintf("%s Downloading %s template to %s", m.spinner.View(), m.packageManager, resolvedPath)
}

func downloadingNewProjectTemplateUpdate(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	if !m.downloadingNewProjectTemplateViewLoaded {
		m.downloadingNewProjectTemplateViewLoaded = true
		return m, tea.Batch(m.spinner.Tick, downloadNewProjectTemplate)
	}

	switch msg := msg.(type) {
	case downloadingNewProjectTemplateDone:
		return m, tea.Quit

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

type downloadingNewProjectTemplateDone bool

func downloadNewProjectTemplate() tea.Msg {
	time.Sleep(1500 * time.Millisecond)
	return downloadingNewProjectTemplateDone(true)
}
