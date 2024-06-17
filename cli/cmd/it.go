package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"gas/degit"
	"gas/helpers"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
	selectInstallPackagesOption   selectInstallPackagesOption
	installingPackages            installingPackages
}

type state string

type enterDirPath struct {
	input textinput.Model
}

type selectEmptyDirOption struct {
	input selectModel
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

type selectInstallPackagesOption struct {
	input selectModel
}

type installingPackages struct {
	viewLoaded bool
}

const (
	_enterDirPath                  state = "_enterDirPath"
	_selectEmptyDirOption          state = "_selectEmptyDirOption"
	_emptyingDirPath               state = "_emptyingDirPath"
	_selectingPackageManager       state = "_selectingPackageManager"
	_downloadingNewProjectTemplate state = "_downloadingNewProjectTemplate"
	_selectInstallPackagesOption   state = "_selectInstallPackagesOption"
	_installingPackages            state = "_installingPackages"
)

func initialModel() model {
	enterDirPathInput := textinput.New()
	enterDirPathInput.Placeholder = "example"
	enterDirPathInput.Focus()

	selectEmptyDirOptionInput := NewSelect()
	selectEmptyDirOptionInput.values = []string{"No", "Yes"}
	selectEmptyDirOptionInput.value = selectEmptyDirOptionInput.values[selectEmptyDirOptionInput.cursor]

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("202"))

	selectPackageManagerInput := NewSelect()
	selectPackageManagerInput.values = []string{"npm", "pnpm"}
	selectPackageManagerInput.value = selectPackageManagerInput.values[selectPackageManagerInput.cursor]

	selectInstallPackagesOptionInput := NewSelect()
	selectInstallPackagesOptionInput.values = []string{"Yes", "No"}
	selectInstallPackagesOptionInput.value = selectInstallPackagesOptionInput.values[selectInstallPackagesOptionInput.cursor]

	return model{
		state:                         _enterDirPath,
		spinner:                       s,
		enterDirPath:                  enterDirPath{input: enterDirPathInput},
		selectEmptyDirOption:          selectEmptyDirOption{input: selectEmptyDirOptionInput},
		emptyingDirPath:               emptyingDirPath{viewLoaded: false},
		selectPackageManager:          selectPackageManager{input: selectPackageManagerInput},
		downloadingNewProjectTemplate: downloadingNewProjectTemplate{viewLoaded: false},
		selectInstallPackagesOption:   selectInstallPackagesOption{input: selectInstallPackagesOptionInput},
		installingPackages:            installingPackages{viewLoaded: false},
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
	case _selectInstallPackagesOption:
		return selectInstallPackagesOptionView(m)
	case _installingPackages:
		return installingPackagesView(m)
	default:
		return ""
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
	case _selectInstallPackagesOption:
		return selectInstallPackagesOptionUpdate(m, msg)
	case _installingPackages:
		return installingPackagesUpdate(m, msg)
	default:
		return m, nil
	}
}

type updateNextStateEvent bool

func updateNextState() tea.Msg {
	return updateNextStateEvent(true)
}

func enterDirPathView(m model) string {
	s := fmt.Sprintf("Directory path:\n\n%s\n\n", m.enterDirPath.input.View())

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
			// Show error if empty
			if m.enterDirPath.input.Value() == "" {
				m.enterDirPath.input.Err = &InputErr{
					Msg: "Directory path is required",
				}
				return m, nil
			}

			// Check if directory exists
			dirPathInputExists, err := helpers.CheckIfDirExists(m.enterDirPath.input.Value())
			if err != nil {
				m.enterDirPath.input.Err = err
				return m, nil
			}

			// Check if directory is empty
			if dirPathInputExists {
				dirPathEntries, err := os.ReadDir(m.enterDirPath.input.Value())
				if err != nil {
					m.enterDirPath.input.Err = err
					return m, nil
				}

				// If directory is not empty, show option to empty it
				if len(dirPathEntries) > 0 {
					m.state = _selectEmptyDirOption
					m.enterDirPath.input.Blur()
					return m, nil
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
	s := fmt.Sprintf(
		"%s is not empty.\n\nEmpty it and continue?\n\n%s\n\n",
		resolvedPath,
		m.selectEmptyDirOption.input.View(),
	)
	s += helpView()
	return s
}

func selectEmptyDirOptionUpdate(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if m.selectEmptyDirOption.input.value == "Yes" {
				m.state = _emptyingDirPath
				return m, updateNextState
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.selectEmptyDirOption.input, cmd = m.selectEmptyDirOption.input.Update(msg)
	return m, cmd
}

type emptyingDirPathDone bool

func emptyDir(dirPath string) tea.Cmd {
	return func() tea.Msg {
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			return emptyingDirPathDone(false)
		}

		for _, entry := range entries {
			err := os.RemoveAll(filepath.Join(dirPath, entry.Name()))
			if err != nil {
				return emptyingDirPathDone(false)
			}
		}
		return emptyingDirPathDone(true)
	}
}

func emptyingDirView(m model) string {
	return fmt.Sprintf("%s Emptying %s", m.spinner.View(), m.enterDirPath.input.Value())
}

func emptyingDirUpdate(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	if !m.emptyingDirPath.viewLoaded {
		m.emptyingDirPath.viewLoaded = true
		return m, tea.Batch(m.spinner.Tick, emptyDir(m.enterDirPath.input.Value()))
	}

	switch msg.(type) {
	case emptyingDirPathDone:
		m.state = _selectingPackageManager
		return m, nil
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func selectingPackageManagerView(m model) string {
	s := "Emptied "
	s += m.enterDirPath.input.Value()
	s += "\n\n"
	s += "Select package manager:\n\n"
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
		if i < len(m.values)-1 {
			s.WriteString("\n")
		}
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
	return fmt.Sprintf(
		"%s Downloading %s template to %s",
		m.spinner.View(),
		m.selectPackageManager.input.value,
		resolvedPath,
	)
}

func downloadingNewProjectTemplateUpdate(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	if !m.downloadingNewProjectTemplate.viewLoaded {
		m.downloadingNewProjectTemplate.viewLoaded = true
		return m, tea.Batch(
			m.spinner.Tick,
			downloadNewProjectTemplate(
				m.enterDirPath.input.Value(),
				m.selectPackageManager.input.value,
			),
		)
	}

	switch msg.(type) {
	case downloadingNewProjectTemplateOk:
		m.state = _selectInstallPackagesOption
		return m, nil
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func downloadNewProjectTemplate(dirPath string, packageManager string) tea.Cmd {
	return func() tea.Msg {
		repoUrl := "https://github.com/gasoline-dev/gasoline"
		repoBranch := "main"
		extractPath := dirPath
		repoTemplate := fmt.Sprintf("templates/new-project-%s", packageManager)

		err := degit.Run(repoUrl, repoBranch, extractPath, repoTemplate)
		if err != nil {
			return downloadingNewProjectTemplateErr(err)
		}

		packageJsonPath := filepath.Join("it", "package.json")

		packageJsonFile, err := os.ReadFile(packageJsonPath)
		if err != nil {
			errMsg := fmt.Errorf("error reading package.json: %w", err)
			return downloadingNewProjectTemplateErr(errMsg)
		}

		var packageJson orderedmap.OrderedMap
		if err := json.Unmarshal(packageJsonFile, &packageJson); err != nil {
			fmt.Println("Error unmarshalling package.json:", err)
			os.Exit(1)
		}

		packageJson.Set("name", "root")

		cmd := exec.Command("npm", "--version")
		output, err := cmd.Output()
		if err != nil {
			fmt.Println("Error getting npm version:", err)
			os.Exit(1)
		}
		packageManagerVersion := strings.TrimSpace(string(output))
		majorVersion := strings.Split(packageManagerVersion, ".")[0]
		packageJson.Set("packageManager", fmt.Sprintf("^npm@%s.0.0", majorVersion))

		updatedPackageJson, err := json.MarshalIndent(packageJson, "", "  ")
		if err != nil {
			fmt.Println("Error marshalling updated package.json:", err)
			os.Exit(1)
		}

		if err := os.WriteFile(packageJsonPath, updatedPackageJson, 0644); err != nil {
			fmt.Println("Error writing updated package.json:", err)
			os.Exit(1)
		}

		gitkeepPath := filepath.Join("./it/gas", ".gitkeep")
		if err := os.Remove(gitkeepPath); err != nil {
			fmt.Println("Error removing .gitkeep:", err)
			os.Exit(1)
		}

		return downloadingNewProjectTemplateOk(true)
	}
}

type downloadingNewProjectTemplateOk bool

type downloadingNewProjectTemplateErr error

func selectInstallPackagesOptionView(m model) string {
	s := fmt.Sprintf("Install packages?\n\n%s\n\n", m.selectInstallPackagesOption.input.View())
	s += helpView()
	return s
}

func selectInstallPackagesOptionUpdate(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if m.selectInstallPackagesOption.input.value == "Yes" {
				m.state = _installingPackages
				return m, updateNextState
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.selectInstallPackagesOption.input, cmd = m.selectInstallPackagesOption.input.Update(msg)
	return m, cmd
}

func installingPackagesView(m model) string {
	return fmt.Sprintf("%s Installing packages", m.spinner.View())
}

func installingPackagesUpdate(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	if !m.installingPackages.viewLoaded {
		m.installingPackages.viewLoaded = true
		return m, tea.Batch(m.spinner.Tick, installPackages)
	}

	switch msg.(type) {
	case installingPackagesDone:
		return m, tea.Quit
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

type installingPackagesDone bool

func installPackages() tea.Msg {
	cmd := exec.Command("npm", "install")
	cmd.Dir = "./it"
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run npm install: %w", err)
	}
	return installingPackagesDone(true)
}
