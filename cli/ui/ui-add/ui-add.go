package uiadd

import (
	"fmt"
	uicommon "gas/ui/ui-common"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Model struct {
	state    state
	ctx      ctx
	list     list.Model
	choice   string
	quitting bool
}

type state string

const (
	selectTemplateState               state = "SELECT_TEMPLATE_STATE"
	selectDownloadTemplateOptionState state = "SELECT_DOWNLOAD_TEMPLATE_OPTION_STATE"
	downloadingTemplateState          state = "DOWNLOADING_TEMPLATE_STATE"
)

type ctx struct {
	list                         list.Model
	templateInput                textinput.Model
	selectDownloadTemplateOption uicommon.SelectModel
}

func InitialModel() Model {
	items := []list.Item{
		item{id: "cloudflare-pages-remix-empty", value: "Cloudflare Pages - Remix - Empty"},
		item{id: "2", value: "Tomato Soup"},
		item{id: "3", value: "Hamburgers"},
		item{id: "4", value: "Cheeseburgers"},
		item{id: "5", value: "Currywurst"},
		item{id: "6", value: "Okonomiyaki"},
		item{id: "7", value: "Pasta"},
		item{id: "8", value: "Fillet Mignon"},
		item{id: "9", value: "Caviar"},
		item{id: "10", value: "Just Wine"},
	}

	const defaultWidth = 20

	l := list.New(items, itemDelegate{}, defaultWidth, listHeight)
	l.Title = "Select template:"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle
	l.Styles.PaginationStyle = paginationStyle
	l.Styles.HelpStyle = helpStyle

	return Model{
		state: selectTemplateState,
		ctx: ctx{
			list:                         l,
			templateInput:                textinput.New(),
			selectDownloadTemplateOption: uicommon.NewSelect(),
		},
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.state {
		case selectTemplateState:
			return selectTemplateUpdate(m, msg)
		case selectDownloadTemplateOptionState:
			return selectDownloadTemplateOptionUpdate(m, msg)
		case downloadingTemplateState:
			return downloadingTemplateUpdate(m, msg)
		}
	}
	return m, nil
}

func (m Model) View() string {
	switch m.state {
	case selectTemplateState:
		return selectTemplateView(m)
	case selectDownloadTemplateOptionState:
		return selectDownloadTemplateOptionView(m)
	case downloadingTemplateState:
		return downloadingTemplateView(m)
	default:
		return "Unknown state view"
	}
}

func selectTemplateUpdate(m Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, tea.ClearScreen

	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "enter":
			i, ok := m.list.SelectedItem().(item)
			if ok {
				m.choice = i.value
			}
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func selectTemplateView(m Model) string {
	if m.choice != "" {
		return quitTextStyle.Render(fmt.Sprintf("%s? Sounds good to me.", m.choice))
	}
	if m.quitting {
		return quitTextStyle.Render("Not hungry? That’s cool.")
	}
	return "\n" + m.list.View()
}

func selectDownloadTemplateOptionUpdate(m Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.ctx.selectDownloadTemplateOption, cmd = m.ctx.selectDownloadTemplateOption.Update(msg)
	return m, cmd
}

func selectDownloadTemplateOptionView(m Model) string {
	return "Download template?\n\n" + m.ctx.selectDownloadTemplateOption.View() + "\n\nPress esc to exit\n\n"
}

func downloadingTemplateUpdate(m Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	return m, cmd
}

func downloadingTemplateView(m Model) string {
	return "Downloading template...\n\nPress esc to exit\n\n"
}

const listHeight = 14

var (
	titleStyle        = lipgloss.NewStyle().MarginLeft(2)
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
	paginationStyle   = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	helpStyle         = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
	quitTextStyle     = lipgloss.NewStyle().Margin(1, 0, 2, 4)
)

type item struct {
	id    string
	value string
}

func (i item) FilterValue() string { return i.value }

type itemDelegate struct{}

func (d itemDelegate) Height() int                             { return 1 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(item)
	if !ok {
		return
	}

	str := string(i.value)

	fn := itemStyle.Render
	if index == m.Index() {
		fn = func(s ...string) string {
			return selectedItemStyle.Render("> " + strings.Join(s, " "))
		}
	}

	fmt.Fprint(w, fn(str))
}
