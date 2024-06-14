package cmd

import (
	"errors"
	"fmt"
	"gas/helpers"
	"os"
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
	selectingPackageManager                 selectModel
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

	return model{
		state:                                   _dirPathInput,
		dirPathInput:                            dirPathInput,
		confirmEmptyDirPathInput:                confirmEmptyDirPathInput,
		emptyingDirPathViewLoaded:               false,
		spinner:                                 s,
		selectingPackageManager:                 selectPackageManager,
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
	return "Press esc or q to exit\n"
}

func confirmEmptyDirPathInputView(m model) string {
	s := fmt.Sprintf("%s is not empty. Empty it? (y/n)\n\n%s\n\n", m.dirPathInput.Value(), m.confirmEmptyDirPathInput.View())
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
	return fmt.Sprintf("emptying dir view %s", m.spinner.View())
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

func selectingPackageManagerView(m model) string {
	return m.selectingPackageManager.View()
}

type testEventDone bool

func testEvent() tea.Msg {
	//time.Sleep(250 * time.Millisecond)
	return testEventDone(true)
}

func selectingPackageManagerUpdate(m model, msg tea.Msg) (tea.Model, tea.Cmd) {

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			m.state = _downloadingNewProjectTemplate

			return m, testEvent
		}
	}

	var cmd tea.Cmd
	m.selectingPackageManager, cmd = m.selectingPackageManager.Update(msg)
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
	s.WriteString("What kind of Bubble Tea would you like to order?\n\n")

	for i := 0; i < len(m.choices); i++ {
		if m.cursor == i {
			s.WriteString("(•) ")
		} else {
			s.WriteString("( ) ")
		}
		s.WriteString(m.choices[i])
		s.WriteString("\n")
	}
	s.WriteString("\n(press q to quit)\n")

	return s.String()
}

func (m selectModel) Update(msg tea.Msg) (selectModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit

		//case "enter":
		//m.choice = m.choices[m.cursor]
		//return m, nil

		case "down", "j":
			m.cursor++
			if m.cursor >= len(m.choices) {
				m.cursor = 0
				m.choice = m.choices[m.cursor]
			}

		case "up", "k":
			m.cursor--
			if m.cursor < 0 {
				m.cursor = len(m.choices) - 1
				m.choice = m.choices[m.cursor]
			}
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
	return fmt.Sprintf("Downloading %s template to %s %s", m.packageManager, m.dirPathInput.Value(), m.spinner.View())
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
