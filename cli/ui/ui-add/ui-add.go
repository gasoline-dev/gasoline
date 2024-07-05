package uiadd

import (
	"fmt"
	uicommon "gas/ui/ui-common"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	state state
	ctx   ctx
}

type state string

const (
	SELECT_TEMPLATE                 state = "SELECT_TEMPLATE"
	ENTER_ENTITY_GROUP_INPUT        state = "ENTER_ENTITY_GROUP_INPUT"
	SELECT_ENTITY_GROUP_LIST        state = "SELECT_ENTITY_GROUP_LIST"
	ENTER_ENTITY_INPUT              state = "ENTER_ENTITY_INPUT"
	SELECT_ENTITY_LIST              state = "SELECT_ENTITY_LIST"
	SELECT_DOWNLOAD_TEMPLATE_OPTION state = "SELECT_DOWNLOAD_TEMPLATE_OPTION"
	DOWNLOADING_TEMPLATE            state = "DOWNLOADING_TEMPLATE"
	FINAL                           state = "FINAL"
)

type ctx struct {
	selectTemplateList           selectTemplateListModel
	entityInput                  textinput.Model
	selectDownloadTemplateOption uicommon.SelectModel
	//selectDownloadTemplateOption uicommon.SelectModel
}

func InitialModel() model {
	items := []list.Item{
		item{id: "cloudflare-pages-remix-empty", value: "Cloudflare Pages - Remix - Empty", entityGroup: "web"},
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

	selectDownloadTemplateOption := uicommon.NewSelect()
	selectDownloadTemplateOption.Options = []uicommon.SelectOption{
		{Id: "yes", Value: "Yes"},
		{Id: "back", Value: "Go back (select template)"},
	}
	selectDownloadTemplateOption.SelectedId = selectDownloadTemplateOption.Options[selectDownloadTemplateOption.Cursor].Id

	entityInput := textinput.New()
	entityInput.Placeholder = "app, dashboard, landing, etc"

	return model{
		state: SELECT_TEMPLATE,
		ctx: ctx{
			selectTemplateList: selectTemplateList,
			entityInput:        entityInput,
		},
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
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
		m.state = FINAL
		if !strings.Contains(string(m.state), "ing") {
			return m, tea.Quit
		}
	}

	switch m.state {
	case SELECT_TEMPLATE:
		return selectTemplateUpdate(m, msg)
	case ENTER_ENTITY_INPUT:
		return enterEntityInputUpdate(m, msg)
	case SELECT_DOWNLOAD_TEMPLATE_OPTION:
		return selectDownloadTemplateOptionUpdate(m, msg)
	}
	return m, nil
}

func (m model) View() string {
	switch m.state {
	case SELECT_TEMPLATE:
		return selectTemplateView(m)
	case ENTER_ENTITY_INPUT:
		return enterEntityInputView(m)
	case SELECT_DOWNLOAD_TEMPLATE_OPTION:
		return selectDownloadTemplateOptionView(m)
	}
	return "Unknown state view"
}

func selectTemplateUpdate(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if m.ctx.selectTemplateList.SelectedItem().entityGroup == "web" {
				m.state = ENTER_ENTITY_INPUT
				return m, tea.Sequence(uicommon.NextState, m.ctx.entityInput.Focus())
			}
			m.state = SELECT_ENTITY_GROUP_LIST
			return m, uicommon.NextState
		}
	}

	var cmd tea.Cmd
	m.ctx.selectTemplateList, cmd = m.ctx.selectTemplateList.Update(msg)
	return m, cmd
}

func selectTemplateView(m model) string {
	return m.ctx.selectTemplateList.View()
}

func enterEntityInputUpdate(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.ctx.entityInput, cmd = m.ctx.entityInput.Update(msg)
	return m, cmd
}

func enterEntityInputView(m model) string {
	s := fmt.Sprintf(
		"Selected the \"%s\" template.\n\n",
		m.ctx.selectTemplateList.SelectedItem().value,
	)
	s += "Enter entity:\n\n"
	s += m.ctx.entityInput.View()
	return s
}

func selectDownloadTemplateOptionUpdate(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func selectDownloadTemplateOptionView(m model) string {
	s := fmt.Sprintf("%s selected.\n\n", m.ctx.selectTemplateList.SelectedId())
	s += "Download it?\n\n"
	s += m.ctx.selectDownloadTemplateOption.View()
	return s
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

func (l selectTemplateListModel) SelectedItem() item {
	return l.Items()[l.cursor].(item)
}

type item struct {
	id          string
	value       string
	entityGroup string
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
