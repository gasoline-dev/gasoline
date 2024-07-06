package uiadd

import (
	"errors"
	"fmt"
	uicommon "gas/ui/ui-common"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	SELECT_TEMPLATE          = "SELECT_TEMPLATE"
	ENTER_ENTITY_GROUP_INPUT = "ENTER_ENTITY_GROUP_INPUT" // TODO: implement
	SELECT_ENTITY_GROUP_LIST = "SELECT_ENTITY_GROUP_LIST" // TODO: implement
	ENTER_ENTITY_INPUT       = "ENTER_ENTITY_INPUT"
	SELECT_ENTITY_LIST       = "SELECT_ENTITY_LIST" // TODO: implement
	ADDED_TEMPLATE           = "ADDED_TEMPLATE"
)

var ui = uicommon.New()

type model struct {
	state              string
	selectTemplateList selectTemplateListModel
	entityInput        textinput.Model
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

	entityInput := textinput.New()
	entityInput.Placeholder = "app, dashboard, landing, etc"

	return model{state: SELECT_TEMPLATE, selectTemplateList: selectTemplateList, entityInput: entityInput}
}

func (m model) Init() tea.Cmd {
	ui.Register(SELECT_TEMPLATE, uicommon.Fns{Update: selectTemplateUpdate, View: selectTemplateView})
	ui.Register(ENTER_ENTITY_INPUT, uicommon.Fns{Update: enterEntityInputUpdate, View: enterEntityInputView})
	ui.Register(ADDED_TEMPLATE, uicommon.Fns{Update: addedTemplateUpdate, View: addedTemplateView})

	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := uicommon.HandleMsgs(msg, m.state)
	if cmd != nil {
		return m, cmd
	}

	uiFn, ok := ui.Fns[m.state]
	if !ok {
		return m, nil
	}
	return uiFn.Update(m, msg)
}

func (m model) View() string {
	uiFn, ok := ui.Fns[m.state]
	if !ok {
		return "unknown"
	}
	return uiFn.View(m)
}

func selectTemplateUpdate(m tea.Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	model := m.(model)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if model.selectTemplateList.SelectedItem().entityGroup == "web" {
				model.state = ENTER_ENTITY_INPUT
				return model, tea.Sequence(uicommon.NextState, model.entityInput.Focus())
			}
			model.state = SELECT_ENTITY_GROUP_LIST
			return model, uicommon.NextState
		}
	}

	var cmd tea.Cmd
	model.selectTemplateList, cmd = model.selectTemplateList.Update(msg)
	return model, cmd
}

func selectTemplateView(m tea.Model) string {
	model := m.(model)
	return model.selectTemplateList.View()
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

func enterEntityInputUpdate(m tea.Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	model := m.(model)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "enter" {
			if model.entityInput.Value() == "" {
				model.entityInput.Err = &uicommon.InputErr{
					Msg: "Entity is required",
				}
				return model, nil
			}
			model.state = ADDED_TEMPLATE
			return model, uicommon.NextState
		}
	}

	var cmd tea.Cmd
	model.entityInput, cmd = model.entityInput.Update(msg)
	return model, cmd
}

func enterEntityInputView(m tea.Model) string {
	model := m.(model)
	s := fmt.Sprintf(
		"Selected \"%s\" template.\n\n",
		model.selectTemplateList.SelectedItem().value,
	)
	s += "Enter entity:\n\n"
	s += fmt.Sprintf("%s\n\n", model.entityInput.View())
	if model.entityInput.Err != nil {
		var inputErr *uicommon.InputErr
		switch {
		case errors.As(model.entityInput.Err, &inputErr):
			s += fmt.Sprintf("%v\n\n", model.entityInput.Err)
		default:
			s += fmt.Sprintf("Error: %v\n\n", model.entityInput.Err)
		}
	}
	s += uicommon.EscView()
	return s
}

func addedTemplateUpdate(m tea.Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	model := m.(model)

	return model, nil
}

func addedTemplateView(m tea.Model) string {
	model := m.(model)
	s := fmt.Sprintf(
		"Template \"%s\" has been added successfully.\n\n",
		model.selectTemplateList.SelectedItem().value,
	)
	s += "Press enter to exit.\n\n"
	return s
}
