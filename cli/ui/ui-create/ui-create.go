package uicreate

import (
	"encoding/json"
	"errors"
	"fmt"
	"gas/degit"
	"gas/helpers"
	uicommon "gas/ui/ui-common"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/iancoleman/orderedmap"
)

type Model struct {
	state    state
	spinner  spinner.Model
	ctx      ctx
	Logs     []string
	LogsTest LogsTestType
}

type state string

const (
	ENTER_DIR_PATH_STATE                   state = "ENTER_DIR_PATH_STATE"
	CREATE_DIR_STATE                       state = "CREATE_DIR_STATE"
	SELECT_EMPTY_DIR_PATH_OPTION_STATE     state = "SELECT_EMPTY_DIR_PATH_OPTION_STATE"
	EMPTYING_DIR_PATH_STATE                state = "EMPTYING_DIR_PATH_STATE"
	SELECT_PACKAGE_MANAGER_STATE           state = "SELECT_PACKAGE_MANAGER_STATE"
	DOWNLOADING_NEW_PROJECT_TEMPLATE_STATE state = "DOWNLOADING_NEW_PROJECT_TEMPLATE_STATE"
	SELECT_INSTALL_PACKAGES_OPTION_STATE   state = "SELECT_INSTALL_PACKAGES_OPTION_STATE"
	INSTALLING_PACKAGES_STATE              state = "INSTALLING_PACKAGES_STATE"
	FINAL_STATE                            state = "FINAL_STATE"
)

type ctx struct {
	dirPathInitStatus             dirPathInitStatus
	dirPathInput                  textinput.Model
	dirPathResolved               string
	createDirInput                uicommon.SelectModel
	selectEmptyDirPathOptionInput uicommon.SelectModel
	selectPackageManagerInput     uicommon.SelectModel
	selectInstallPackagesInput    uicommon.SelectModel
}

var logsCollection []string

func GetLogs() []string {
	return logsCollection
}

type dirPathInitStatus string

const (
	DIR_PATH_INIT_STATUS_EMPTY dirPathInitStatus = "EMPTY"
	DIR_PATH_INIT_STATUS_FULL  dirPathInitStatus = "FULL"
	DIR_PATH_INIT_STATUS_NONE  dirPathInitStatus = "NONE"
)

type LogsTestType []string

func InitialModel() Model {
	logsCollection = append(logsCollection, "idk")

	s := spinner.New()
	s.Spinner = spinner.Dot

	dirPathInput := textinput.New()
	dirPathInput.Placeholder = "example"
	dirPathInput.Focus()

	createDirInput := uicommon.NewSelect()
	createDirInput.Options = []uicommon.SelectOption{
		{Id: "yes", Value: "Yes"},
		{Id: "back", Value: "Go back (enter directory path)"},
	}
	createDirInput.SelectedId = createDirInput.Options[createDirInput.Cursor].Id

	selectPackageManagerInput := uicommon.NewSelect()
	selectPackageManagerInput.Options = []uicommon.SelectOption{
		{Id: "npm", Value: "npm"},
	}
	selectPackageManagerInput.SelectedId = selectPackageManagerInput.Options[selectPackageManagerInput.Cursor].Id

	selectEmptyDirPathOptionInput := uicommon.NewSelect()
	selectEmptyDirPathOptionInput.Options = []uicommon.SelectOption{
		{Id: "yes", Value: "Yes"},
		{Id: "back", Value: "Go back (enter directory path)"},
	}
	selectEmptyDirPathOptionInput.SelectedId = selectEmptyDirPathOptionInput.Options[selectEmptyDirPathOptionInput.Cursor].Id

	selectInstallPackagesInput := uicommon.NewSelect()
	selectInstallPackagesInput.Options = []uicommon.SelectOption{
		{Id: "yes", Value: "Yes"},
		{Id: "no", Value: "No"},
	}
	selectInstallPackagesInput.SelectedId = selectInstallPackagesInput.Options[selectInstallPackagesInput.Cursor].Id

	return Model{
		state:   ENTER_DIR_PATH_STATE,
		spinner: s,
		ctx: ctx{
			dirPathInput:                  dirPathInput,
			createDirInput:                createDirInput,
			selectPackageManagerInput:     selectPackageManagerInput,
			selectEmptyDirPathOptionInput: selectEmptyDirPathOptionInput,
			selectInstallPackagesInput:    selectInstallPackagesInput,
		},
		LogsTest: LogsTestType{"initial", "model", "created"},
	}
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m, tea.ClearScreen
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			// User shouldn't be able to exit while state is processing
			if !strings.Contains(string(m.state), "ING") {
				return m, tea.Sequence(tea.ClearScreen, tea.Quit)
			}
		}
	case uicommon.FinalStateType:
		m.state = FINAL_STATE
		if !strings.Contains(string(m.state), "ing") {
			return m, tea.Quit
		}
	}

	switch m.state {
	case ENTER_DIR_PATH_STATE:
		return enterDirPathUpdate(m, msg)
	case CREATE_DIR_STATE:
		return createDirUpdate(m, msg)
	case SELECT_EMPTY_DIR_PATH_OPTION_STATE:
		return selectEmptyDirPathOptionUpdate(m, msg)
	case EMPTYING_DIR_PATH_STATE:
		return emptyingDirPathUpdate(m, msg)
	case SELECT_PACKAGE_MANAGER_STATE:
		return selectPackageManagerUpdate(m, msg)
	case DOWNLOADING_NEW_PROJECT_TEMPLATE_STATE:
		return downloadingNewProjectTemplateUpdate(m, msg)
	case SELECT_INSTALL_PACKAGES_OPTION_STATE:
		return selectInstallPackagesOptionUpdate(m, msg)
	case INSTALLING_PACKAGES_STATE:
		return installingPackagesUpdate(m, msg)
	default:
		return unknownStateUpdate(m)
	}
}

func (m Model) View() string {
	switch m.state {
	case FINAL_STATE:
		s := "\n"
		for _, log := range m.Logs {
			s += fmt.Sprintf("  %s\n", log)
		}
		s += "\n  See you later!\n\n"
		return s
	case ENTER_DIR_PATH_STATE:
		return enterDirPathView(m)
	case CREATE_DIR_STATE:
		return createDirView(m)
	case SELECT_EMPTY_DIR_PATH_OPTION_STATE:
		return selectEmptyDirPathOptionView(m)
	case EMPTYING_DIR_PATH_STATE:
		return emptyingDirPathView(m)
	case SELECT_PACKAGE_MANAGER_STATE:
		return selectPackageManagerView(m)
	case DOWNLOADING_NEW_PROJECT_TEMPLATE_STATE:
		return downloadingNewProjectTemplateView(m)
	case SELECT_INSTALL_PACKAGES_OPTION_STATE:
		return selectInstallPackagesOptionView(m)
	case INSTALLING_PACKAGES_STATE:
		return installingPackagesView(m)
	default:
		return unknownView()
	}
}

func escView() string {
	return "Press esc to exit\n\n"
}

func enterDirPathUpdate(m Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			if m.ctx.dirPathInput.Value() == "" {
				m.ctx.dirPathInput.Err = &uicommon.InputErr{
					Msg: "Directory path is required",
				}
				return m, nil
			}

			resolvedPath, _ := filepath.Abs(m.ctx.dirPathInput.Value())

			m.ctx.dirPathResolved = resolvedPath

			return m, getDirPathInitStatus(m.ctx.dirPathResolved)
		}
	case getDirPathInitStatusOk:
		m.ctx.dirPathInitStatus = dirPathInitStatus(msg)
		if m.ctx.dirPathInitStatus == DIR_PATH_INIT_STATUS_NONE {
			m.state = CREATE_DIR_STATE
		} else if m.ctx.dirPathInitStatus == DIR_PATH_INIT_STATUS_EMPTY {
			m.state = SELECT_PACKAGE_MANAGER_STATE
		} else if m.ctx.dirPathInitStatus == DIR_PATH_INIT_STATUS_FULL {
			m.state = SELECT_EMPTY_DIR_PATH_OPTION_STATE
		}
		return m, uicommon.Tx
	case getDirPathInitStatusErr:
		m.Logs = append(m.Logs, fmt.Sprintf("Error: %v", msg))
		return m, uicommon.FinalState
	}

	var cmd tea.Cmd
	m.ctx.dirPathInput, cmd = m.ctx.dirPathInput.Update(msg)
	return m, cmd
}

func enterDirPathView(m Model) string {
	s := "Enter directory path:\n\n"
	s += fmt.Sprintf("%s\n\n", m.ctx.dirPathInput.View())
	if m.ctx.dirPathInput.Err != nil {
		var inputErr *uicommon.InputErr
		switch {
		case errors.As(m.ctx.dirPathInput.Err, &inputErr):
			s += fmt.Sprintf("%v\n\n", m.ctx.dirPathInput.Err)
		default:
			s += fmt.Sprintf("Error: %v\n\n", m.ctx.dirPathInput.Err)
		}
	}
	s += escView()
	return s
}

type getDirPathInitStatusOk dirPathInitStatus

type getDirPathInitStatusErr error

func getDirPathInitStatus(dirPath string) tea.Cmd {
	return func() tea.Msg {
		dirExists, err := helpers.CheckIfDirExists(dirPath)
		if err != nil {
			return getDirPathInitStatusErr(err)
		}
		if !dirExists {
			return getDirPathInitStatusOk(DIR_PATH_INIT_STATUS_NONE)
		}

		files, err := os.ReadDir(dirPath)
		if err != nil {
			return getDirPathInitStatusErr(err)
		}
		if len(files) == 0 {
			return getDirPathInitStatusOk(DIR_PATH_INIT_STATUS_EMPTY)
		}
		return getDirPathInitStatusOk(DIR_PATH_INIT_STATUS_FULL)
	}
}

func createDirUpdate(m Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if m.ctx.createDirInput.SelectedId == "yes" {
				return m, createDir(m.ctx.dirPathResolved)
			} else if m.ctx.createDirInput.SelectedId == "back" {
				m.state = ENTER_DIR_PATH_STATE
				m.ctx.dirPathInput.Reset()
				m.ctx.createDirInput.Reset()
				return m, tea.Batch(uicommon.Tx, m.ctx.dirPathInput.Focus())
			}
		}
	case createDirOk:
		m.state = SELECT_PACKAGE_MANAGER_STATE
		return m, uicommon.Tx
	case createDirErr:
		m.Logs = append(m.Logs, fmt.Sprintf("Error: %v", msg))
		return m, uicommon.FinalState
	}

	var cmd tea.Cmd
	m.ctx.createDirInput, cmd = m.ctx.createDirInput.Update(msg)
	return m, cmd
}

func createDirView(m Model) string {
	s := fmt.Sprintf("> %s does not exist.\n\n", m.ctx.dirPathResolved)
	s += "Create it?\n\n"
	s += m.ctx.createDirInput.View()
	s += "\n\n"
	s += escView()
	return s
}

type createDirOk bool

type createDirErr error

func createDir(dirPath string) tea.Cmd {
	return func() tea.Msg {
		err := os.Mkdir(dirPath, 0755)
		if err != nil {
			return createDirErr(err)
		}
		return createDirOk(true)
	}
}

func selectEmptyDirPathOptionView(m Model) string {
	s := fmt.Sprintf("> %s is not empty.\n\n", m.ctx.dirPathResolved)
	s += "Empty it?\n\n"
	s += m.ctx.selectEmptyDirPathOptionInput.View()
	s += "\n\n"
	s += escView()
	return s
}

func selectEmptyDirPathOptionUpdate(m Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if m.ctx.selectEmptyDirPathOptionInput.SelectedId == "yes" {
				m.state = EMPTYING_DIR_PATH_STATE
				return m, uicommon.Tx
			} else if m.ctx.selectEmptyDirPathOptionInput.SelectedId == "back" {
				m.state = ENTER_DIR_PATH_STATE
				m.ctx.dirPathInput.Reset()
				m.ctx.selectEmptyDirPathOptionInput.Reset()
				return m, tea.Batch(uicommon.Tx, m.ctx.dirPathInput.Focus())
			}
		}
	}

	var cmd tea.Cmd
	m.ctx.selectEmptyDirPathOptionInput, cmd = m.ctx.selectEmptyDirPathOptionInput.Update(msg)
	return m, cmd
}

func emptyingDirPathView(m Model) string {
	s := fmt.Sprintf("%s Emptying %s...\n\n", m.spinner.View(), m.ctx.dirPathResolved)
	return s
}

func emptyingDirPathUpdate(m Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case uicommon.TxMsg:
		return m, tea.Batch(m.spinner.Tick, emptyDirPath(m.ctx.dirPathResolved))
	case emptyDirPathOk:
		m.state = SELECT_PACKAGE_MANAGER_STATE
		return m, uicommon.Tx
	case emptyDirPathErr:
		m.Logs = append(m.Logs, fmt.Sprintf("Error: %v", msg))
		return m, uicommon.FinalState
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

type emptyDirPathOk bool

type emptyDirPathErr error

func emptyDirPath(dirPath string) tea.Cmd {
	return func() tea.Msg {
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			return emptyDirPathErr(err)
		}

		for _, entry := range entries {
			err := os.RemoveAll(filepath.Join(dirPath, entry.Name()))
			if err != nil {
				return emptyDirPathErr(err)
			}
		}
		return emptyDirPathOk(true)
	}
}

func selectPackageManagerUpdate(m Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			m.state = DOWNLOADING_NEW_PROJECT_TEMPLATE_STATE
			return m, uicommon.Tx
		}
	}

	var cmd tea.Cmd
	m.ctx.selectPackageManagerInput, cmd = m.ctx.selectPackageManagerInput.Update(msg)
	return m, cmd
}

func selectPackageManagerView(m Model) string {
	s := ""
	if m.ctx.dirPathInitStatus == DIR_PATH_INIT_STATUS_NONE {
		s += fmt.Sprintf("> %s created.\n\n", m.ctx.dirPathResolved)
	} else if m.ctx.dirPathInitStatus == DIR_PATH_INIT_STATUS_EMPTY {
		s += fmt.Sprintf("> %s is empty.\n\n", m.ctx.dirPathResolved)
	} else if m.ctx.dirPathInitStatus == DIR_PATH_INIT_STATUS_FULL {
		s += fmt.Sprintf("> %s emptied.\n\n", m.ctx.dirPathResolved)
	}
	s += "We're going to download a starter template next.\n\n"
	s += "Select package manager:\n\n"
	s += m.ctx.selectPackageManagerInput.View()
	s += "\n\n"
	s += escView()
	return s
}

func downloadingNewProjectTemplateUpdate(m Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case uicommon.TxMsg:
		return m, tea.Batch(
			m.spinner.Tick,
			downloadNewProjectTemplate(
				m.ctx.dirPathResolved,
				m.ctx.selectPackageManagerInput.SelectedId,
			),
		)
	case downloadNewProjectTemplateOk:
		m.state = SELECT_INSTALL_PACKAGES_OPTION_STATE
		return m, uicommon.Tx
	case downloadNewProjectTemplateErr:
		m.Logs = append(m.Logs, fmt.Sprintf("Error: %v", msg))
		return m, uicommon.FinalState
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func downloadingNewProjectTemplateView(m Model) string {
	return fmt.Sprintf(
		"%s Downloading %s starter template...\n\n",
		m.spinner.View(),
		m.ctx.selectPackageManagerInput.SelectedId,
	)
}

type downloadNewProjectTemplateOk bool

type downloadNewProjectTemplateErr error

func downloadNewProjectTemplate(dirPath string, packageManager string) tea.Cmd {
	return func() tea.Msg {
		repoUrl := "https://github.com/gasoline-dev/gasoline"
		repoBranch := "main"
		extractPath := dirPath
		repoTemplate := fmt.Sprintf("templates/new-project-%s", packageManager)

		err := degit.Run(repoUrl, repoBranch, extractPath, repoTemplate)
		if err != nil {
			return downloadNewProjectTemplateErr(err)
		}

		gasConfigPath := filepath.Join(extractPath, "gas.config.json")

		gasConfigFile, err := os.ReadFile(gasConfigPath)
		if err != nil {
			errMsg := fmt.Errorf("unable to read template gas.config.json: %w", err)
			return downloadNewProjectTemplateErr(errMsg)
		}

		var gasConfig orderedmap.OrderedMap
		if err := json.Unmarshal(gasConfigFile, &gasConfig); err != nil {
			errMsg := fmt.Errorf("unable to unmarshal template gas.config.json: %w", err)
			return downloadNewProjectTemplateErr(errMsg)
		}

		gasConfig.Set("project", helpers.StringToLowerCaseKebab(filepath.Base(dirPath)))

		updatedGasConfig, err := json.MarshalIndent(gasConfig, "", "  ")
		if err != nil {
			errMsg := fmt.Errorf("unable to marshal updated gas.config.json: %w", err)
			return downloadNewProjectTemplateErr(errMsg)
		}

		if err := os.WriteFile(gasConfigPath, updatedGasConfig, 0644); err != nil {
			errMsg := fmt.Errorf("unable to write updated gas.config.json: %w", err)
			return downloadNewProjectTemplateErr(errMsg)
		}

		packageJsonPath := filepath.Join(extractPath, "package.json")

		packageJsonFile, err := os.ReadFile(packageJsonPath)
		if err != nil {
			errMsg := fmt.Errorf("unable to read template package.json: %w", err)
			return downloadNewProjectTemplateErr(errMsg)
		}

		var packageJson orderedmap.OrderedMap
		if err := json.Unmarshal(packageJsonFile, &packageJson); err != nil {
			errMsg := fmt.Errorf("unable to unmarshal template package.json: %w", err)
			return downloadNewProjectTemplateErr(errMsg)
		}

		packageJson.Set("name", "root")

		cmd := exec.Command(packageManager, "--version")
		output, err := cmd.Output()
		if err != nil {
			errMsg := fmt.Errorf("unable to get %s version: %w", packageManager, err)
			return downloadNewProjectTemplateErr(errMsg)
		}
		packageManagerVersion := strings.TrimSpace(string(output))
		majorVersion := strings.Split(packageManagerVersion, ".")[0]
		packageJson.Set("packageManager", fmt.Sprintf("^%s@%s.0.0", packageManager, majorVersion))

		updatedPackageJson, err := json.MarshalIndent(packageJson, "", "  ")
		if err != nil {
			errMsg := fmt.Errorf("unable to marshal updated template package.json: %w", err)
			return downloadNewProjectTemplateErr(errMsg)
		}

		if err := os.WriteFile(packageJsonPath, updatedPackageJson, 0644); err != nil {
			errMsg := fmt.Errorf("unable to write updated template package.json: %w", err)
			return downloadNewProjectTemplateErr(errMsg)
		}

		gitkeepPath := filepath.Join(extractPath, "gas", ".gitkeep")
		if err := os.Remove(gitkeepPath); err != nil {
			errMsg := fmt.Errorf("unable to remove template gas/.gitkeep: %w", err)
			return downloadNewProjectTemplateErr(errMsg)
		}

		return downloadNewProjectTemplateOk(true)
	}
}

func selectInstallPackagesOptionUpdate(m Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if m.ctx.selectInstallPackagesInput.SelectedId == "yes" {
				m.state = INSTALLING_PACKAGES_STATE
				return m, uicommon.Tx
			} else if m.ctx.selectInstallPackagesInput.SelectedId == "no" {
				m.state = FINAL_STATE
				return m, uicommon.Tx
			}
		}
	}

	var cmd tea.Cmd
	m.ctx.selectInstallPackagesInput, cmd = m.ctx.selectInstallPackagesInput.Update(msg)
	return m, cmd
}

func selectInstallPackagesOptionView(m Model) string {
	s := fmt.Sprintf(
		"> Installed %s starter template to %s.\n\n",
		m.ctx.selectPackageManagerInput.SelectedId,
		m.ctx.dirPathResolved,
	)
	s += "Install packages?\n\n"
	s += m.ctx.selectInstallPackagesInput.View()
	s += "\n\n"
	s += escView()
	return s
}

func installingPackagesUpdate(m Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case uicommon.TxMsg:
		return m, tea.Batch(
			m.spinner.Tick,
			installPackages(
				m.ctx.dirPathResolved,
				m.ctx.selectPackageManagerInput.SelectedId,
			),
		)
	case installPackagesOk:
		m.state = FINAL_STATE
		return m, uicommon.FinalState
	case installPackagesErr:
		m.Logs = append(m.Logs, fmt.Sprintf("Error: %v", msg))
		return m, uicommon.FinalState
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func installingPackagesView(m Model) string {
	return fmt.Sprintf("%s Installing packages...\n\n", m.spinner.View())
}

type installPackagesOk bool

type installPackagesErr error

func installPackages(dirPath string, packageManager string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command(packageManager, "install")
		cmd.Dir = dirPath
		if err := cmd.Run(); err != nil {
			return installPackagesErr(
				fmt.Errorf("unable to complete %s install: %w", packageManager, err),
			)
		}
		return installPackagesOk(true)
	}
}

func unknownStateUpdate(m Model) (tea.Model, tea.Cmd) {
	return m, nil
}

func unknownView() string {
	return "Unknown state view"
}
