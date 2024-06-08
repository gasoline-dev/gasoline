package cmd

import (
	"errors"
	"fmt"
	"gas/helpers"
	"io"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create",
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
	state                     state
	dirPathInput              textinput.Model
	confirmEmptyDirPathInput  textinput.Model
	emptyingDirPathViewLoaded bool
	spinner                   spinner.Model
	selectPackageManagerList  list.Model
	packageManager            string
}

type state string

const (
	_dirPathInput              state = "_dirPathInput"
	_confirmEmptyDirPathInput  state = "_confirmEmptyDirPathInput"
	_emptyingDirPath           state = "_emptyingDirPath"
	_selectPackageManager      state = "_selectPackageManager"
	_downloadingCreateTemplate state = "_downloadingCreateTemplate"
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

	selectPackageManagerListItems := []list.Item{
		item("npm"),
		item("pnpm"),
	}

	const defaultWidth = 20
	const listHeight = 14

	selectPackageManagerList := list.New(selectPackageManagerListItems, itemDelegate{}, defaultWidth, listHeight)
	selectPackageManagerList.SetFilteringEnabled(false)
	selectPackageManagerList.SetShowPagination(false)
	selectPackageManagerList.SetShowHelp(false)
	selectPackageManagerList.SetShowTitle(false)
	selectPackageManagerList.SetShowStatusBar(false)

	return model{
		state:                     _dirPathInput,
		dirPathInput:              dirPathInput,
		confirmEmptyDirPathInput:  confirmEmptyDirPathInput,
		emptyingDirPathViewLoaded: false,
		spinner:                   s,
		selectPackageManagerList:  selectPackageManagerList,
		packageManager:            "",
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
	case _selectPackageManager:
		return selectPackageManagerView(m)
	case _downloadingCreateTemplate:
		return downloadingCreateTemplateView(m)
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
	case _selectPackageManager:
		return selectPackageManagerUpdate(m, msg)
	case _downloadingCreateTemplate:
		return downloadingCreateTemplateUpdate(m, msg)
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
		m.state = _selectPackageManager
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

func selectPackageManagerView(m model) string {
	return fmt.Sprintf("Select package manager:\n\n%s", m.selectPackageManagerList.View())
}

func selectPackageManagerUpdate(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "enter":
			i, ok := m.selectPackageManagerList.SelectedItem().(item)
			if ok {
				m.packageManager = string(i)
				m.state = _downloadingCreateTemplate
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.selectPackageManagerList, cmd = m.selectPackageManagerList.Update(msg)
	return m, cmd
}

type InputErr struct {
	Msg string
}

func (e *InputErr) Error() string {
	return e.Msg
}

var (
	titleStyle        = lipgloss.NewStyle().MarginLeft(2)
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
	paginationStyle   = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	helpStyle         = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
	quitTextStyle     = lipgloss.NewStyle().Margin(1, 0, 2, 4)
)

type item string

func (i item) FilterValue() string { return "" }

type itemDelegate struct{}

func (d itemDelegate) Height() int                             { return 1 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(item)
	if !ok {
		return
	}

	str := fmt.Sprintf("%d. %s", index+1, i)

	fn := itemStyle.Render
	if index == m.Index() {
		fn = func(s ...string) string {
			return selectedItemStyle.Render("> " + strings.Join(s, " "))
		}
	}

	fmt.Fprint(w, fn(str))
}

func downloadingCreateTemplateView(m model) string {
	return fmt.Sprintf("Downloading %s template to %s", m.packageManager, m.dirPathInput.Value())
}

func downloadingCreateTemplateUpdate(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}
