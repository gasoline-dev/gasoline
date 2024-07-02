package uiadd

import (
	"fmt"
	uicommon "gas/ui/ui-common"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	state state
	ctx   ctx
}

type state string

const (
	SELECT_TEMPLATE_STATE                 state = "SELECT_TEMPLATE_STATE"
	SELECT_DOWNLOAD_TEMPLATE_OPTION_STATE state = "SELECT_DOWNLOAD_TEMPLATE_OPTION_STATE"
	DOWNLOADING_TEMPLATE_STATE            state = "DOWNLOADING_TEMPLATE_STATE"
	FINAL_STATE                           state = "FINAL_STATE"
)

type ctx struct {
	selectTemplateList selectTemplateListModel
	//templateInput                textinput.Model
	//selectDownloadTemplateOption uicommon.SelectModel
}

func InitialModel() model {
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

	const listWidth = 20
	const listHeight = 14

	selectTemplateList := newSelectTemplateListModel(items, itemDelegate{}, listWidth, listHeight)

	selectTemplateList.Title = "Select template:"
	selectTemplateList.SetShowStatusBar(false)
	selectTemplateList.SetFilteringEnabled(false)
	selectTemplateList.Styles.Title = titleStyle
	selectTemplateList.Styles.PaginationStyle = paginationStyle
	selectTemplateList.Styles.HelpStyle = helpStyle

	return model{
		state: SELECT_TEMPLATE_STATE,
		ctx: ctx{
			selectTemplateList: selectTemplateList,
		},
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
	case SELECT_TEMPLATE_STATE:
		return selectTemplateUpdate(m, msg)
	case SELECT_DOWNLOAD_TEMPLATE_OPTION_STATE:
		return selectDownloadTemplateOptionUpdate(m, msg)
	}
	return m, nil
}

func (m model) View() string {
	switch m.state {
	case SELECT_TEMPLATE_STATE:
		return selectTemplateView(m)
	case SELECT_DOWNLOAD_TEMPLATE_OPTION_STATE:
		return selectDownloadTemplateOptionView(m)
	}
	return "Unknown state view"
}

func selectTemplateUpdate(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			m.state = SELECT_DOWNLOAD_TEMPLATE_OPTION_STATE
		}
	}

	var cmd tea.Cmd
	m.ctx.selectTemplateList, cmd = m.ctx.selectTemplateList.Update(msg)
	return m, cmd
}

func selectTemplateView(m model) string {
	return m.ctx.selectTemplateList.View()
}

func selectDownloadTemplateOptionUpdate(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func selectDownloadTemplateOptionView(m model) string {
	return fmt.Sprintf("Select download template option: %s", m.ctx.selectTemplateList.SelectedId())
}

var (
	titleStyle        = lipgloss.NewStyle().MarginLeft(2)
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
	paginationStyle   = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	helpStyle         = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
	quitTextStyle     = lipgloss.NewStyle().Margin(1, 0, 2, 4)
)

type selectTemplateListModel struct {
	list.Model
	cursor     int
	selectedId string
}

func newSelectTemplateListModel(items []list.Item,
	delegate list.ItemDelegate,
	width int,
	height int) selectTemplateListModel {
	return selectTemplateListModel{
		Model:      list.New(items, delegate, width, height),
		cursor:     0,
		selectedId: "",
	}
}

func (l selectTemplateListModel) init() tea.Cmd {
	return nil
}

func (m selectTemplateListModel) Update(msg tea.Msg) (selectTemplateListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "down", "j":
			m.cursor++
			if m.cursor >= len(m.Items()) {
				m.cursor = 0
			}
			m.selectedId = m.Items()[m.cursor].(item).id

		case "up", "k":
			m.cursor--
			if m.cursor < 0 {
				m.cursor = len(m.Items()) - 1
			}
			m.selectedId = m.Items()[m.cursor].(item).id

		case "tab":
			if m.cursor == len(m.Items())-1 {
				m.cursor = 0
			} else {
				m.cursor++
			}
			m.selectedId = m.Items()[m.cursor].(item).id
		}
	}

	var cmd tea.Cmd
	m.Model, cmd = m.Model.Update(msg)
	return m, cmd
}

func (m selectTemplateListModel) View() string {
	return m.Model.View()
}

func (l selectTemplateListModel) SelectedId() string {
	return l.selectedId
}

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
