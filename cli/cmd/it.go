package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"gas/degit"
	"gas/helpers"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/iancoleman/orderedmap"
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
	state                         state
	spinner                       spinner.Model
	enterDirPath                  enterDirPath
	selectEmptyDirOption          selectEmptyDirOption
	emptyingDirPath               emptyingDirPath
	selectPackageManager          selectPackageManager
	downloadingNewProjectTemplate downloadingNewProjectTemplate
}

type enterDirPath struct {
	input textinput.Model
}

type selectEmptyDirOption struct {
	input textinput.Model
}

type emptyingDirPath struct {
	viewLoaded bool
}

type selectPackageManager struct {
	input selectModel
}

type downloadingNewProjectTemplate struct {
	viewLoaded bool
}

type state string

const (
	_enterDirPath                  state = "_enterDirPath"
	_selectEmptyDirOption          state = "_selectEmptyDirOption"
	_emptyingDirPath               state = "_emptyingDirPath"
	_selectingPackageManager       state = "_selectingPackageManager"
	_downloadingNewProjectTemplate state = "_downloadingNewProjectTemplate"
)

func initialModel() model {
	enterDirPathInput := textinput.New()
	enterDirPathInput.Placeholder = "./example"
	enterDirPathInput.Focus()

	selectEmptyDirOptionInput := textinput.New()
	selectEmptyDirOptionInput.CharLimit = 1

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("202"))

	selectPackageManagerInput := NewSelect()
	selectPackageManagerInput.values = []string{"npm", "pnpm"}
	selectPackageManagerInput.value = selectPackageManagerInput.values[selectPackageManagerInput.cursor]

	return model{
		state:                         _enterDirPath,
		enterDirPath:                  enterDirPath{input: enterDirPathInput},
		selectEmptyDirOption:          selectEmptyDirOption{input: selectEmptyDirOptionInput},
		emptyingDirPath:               emptyingDirPath{viewLoaded: false},
		spinner:                       s,
		selectPackageManager:          selectPackageManager{input: selectPackageManagerInput},
		downloadingNewProjectTemplate: downloadingNewProjectTemplate{viewLoaded: false},
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) View() string {
	switch m.state {
	case _enterDirPath:
		return enterDirPathView(m)
	case _selectEmptyDirOption:
		return selectEmptyDirOptionView(m)
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
	case _enterDirPath:
		return enterDirPathUpdate(m, msg)
	case _selectEmptyDirOption:
		return selectEmptyDirOptionUpdate(m, msg)
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

func enterDirPathView(m model) string {
	s := fmt.Sprintf(
		"Directory path:\n\n%s\n\n",
		m.enterDirPath.input.View())

	if m.enterDirPath.input.Err != nil {
		var inputErr *InputErr
		switch {
		case errors.As(m.enterDirPath.input.Err, &inputErr):
			s += fmt.Sprintf("%v\n\n", m.enterDirPath.input.Err)
		default:
			s += fmt.Sprintf("Error: %v\n\n", m.enterDirPath.input.Err)
		}
	}

	s += helpView()

	return s
}

func enterDirPathUpdate(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			if m.enterDirPath.input.Value() == "" {
				m.enterDirPath.input.Err = &InputErr{
					Msg: "Directory path is required",
				}
				return m, nil
			}

			dirPathInputExists, err := helpers.CheckIfDirExists(m.enterDirPath.input.Value())
			if err != nil {
				m.enterDirPath.input.Err = err
				return m, nil
			}

			if dirPathInputExists {
				dirPathEntries, err := os.ReadDir(m.enterDirPath.input.Value())
				if err != nil {
					m.enterDirPath.input.Err = err
					return m, nil
				}

				if len(dirPathEntries) > 0 {
					m.state = _selectEmptyDirOption
					m.enterDirPath.input.Blur()
					return m, m.selectEmptyDirOption.input.Focus()
				}
			}

			m.state = _selectingPackageManager
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.enterDirPath.input, cmd = m.enterDirPath.input.Update(msg)
	return m, cmd
}

func helpView() string {
	return "Press esc to exit\n"
}

func selectEmptyDirOptionView(m model) string {
	resolvedPath, _ := filepath.Abs(m.enterDirPath.input.Value())
	s := fmt.Sprintf("%s is not empty.\n\nEmpty it? (y/n)\n\n%s\n\n", resolvedPath, m.selectEmptyDirOption.input.View())
	s += helpView()
	return s
}

func selectEmptyDirOptionUpdate(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "enter":
			lowercaseValue := strings.ToLower(m.selectEmptyDirOption.input.Value())
			if lowercaseValue == "y" {
				m.state = _emptyingDirPath
				return m, nil
			}
			return m, nil
		}
	}

	var cmd tea.Cmd

	m.selectEmptyDirOption.input, cmd = m.selectEmptyDirOption.input.Update(msg)

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
	if !m.emptyingDirPath.viewLoaded {
		m.emptyingDirPath.viewLoaded = true
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
	s += m.selectPackageManager.input.View()
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
			return m, updateNextState
		}
	}

	var cmd tea.Cmd
	m.selectPackageManager.input, cmd = m.selectPackageManager.input.Update(msg)
	return m, cmd
}

type selectModel struct {
	cursor int
	value  string
	values []string
}

func NewSelect() selectModel {
	return selectModel{}
}

func (m selectModel) Init() tea.Cmd {
	return nil
}

func (m selectModel) View() string {
	s := strings.Builder{}

	for i := 0; i < len(m.values); i++ {
		if m.cursor == i {
			s.WriteString("(•) ")
		} else {
			s.WriteString("( ) ")
		}
		s.WriteString(m.values[i])
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
			if m.cursor >= len(m.values) {
				m.cursor = 0
			}
			m.value = m.values[m.cursor]

		case "up", "k":
			m.cursor--
			if m.cursor < 0 {
				m.cursor = len(m.values) - 1
			}
			m.value = m.values[m.cursor]

		case "tab":
			if m.cursor == len(m.values)-1 {
				m.cursor = 0
			} else {
				m.cursor++
			}
			m.value = m.values[m.cursor]
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
	resolvedPath, _ := filepath.Abs(m.enterDirPath.input.Value())
	return fmt.Sprintf("%s Downloading %s template to %s", m.spinner.View(), m.selectPackageManager.input.value, resolvedPath)
}

func downloadingNewProjectTemplateUpdate(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	if !m.downloadingNewProjectTemplate.viewLoaded {
		m.downloadingNewProjectTemplate.viewLoaded = true
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

func downloadNewProjectTemplate() tea.Msg {
	err := degit.Run("https://github.com/gasoline-dev/gasoline", "main", "it", "templates/new-project-npm")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	packageJsonPath := filepath.Join("it", "package.json")

	packageJsonFile, err := os.ReadFile(packageJsonPath)
	if err != nil {
		fmt.Println("Error reading package.json:", err)
		os.Exit(1)
	}

	var packageJson orderedmap.OrderedMap
	if err := json.Unmarshal(packageJsonFile, &packageJson); err != nil {
		fmt.Println("Error unmarshalling package.json:", err)
		os.Exit(1)
	}

	packageJson.Set("name", "Test")

	updatedPackageJson, err := json.MarshalIndent(packageJson, "", "  ")
	if err != nil {
		fmt.Println("Error marshalling updated package.json:", err)
		os.Exit(1)
	}

	if err := os.WriteFile(packageJsonPath, updatedPackageJson, 0644); err != nil {
		fmt.Println("Error writing updated package.json:", err)
		os.Exit(1)
	}

	return downloadingNewProjectTemplateDone(true)
}

type downloadingNewProjectTemplateDone bool
