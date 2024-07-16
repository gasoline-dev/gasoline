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

type themeType struct {
	header lipgloss.Style
}

func theme() *themeType {
	var t themeType

	t.header = lipgloss.NewStyle().
		Background(lipgloss.Color("8")).
		Width(11).
		AlignHorizontal(lipgloss.Center).
		Bold(true).
		Margin(1, 0, 0, 1)

	return &t
}

const (
	SELECT_TEMPLATE = "SELECT_TEMPLATE"
	ENTER_ENTITY    = "ENTER_ENTITY"
)

type model struct {
	state          string
	selectTemplate selectTemplate
	enterEntity    enterEntity
}

type selectTemplate struct {
	list list.Model
}

type enterEntity struct {
	input textinput.Model
}

func InitialModel() model {
	templates := []list.Item{
		item{
			id:          "cloudflare-pages-remix-empty",
			value:       "Cloudflare Pages - Remix - Empty",
			entityGroup: "web",
			entity:      "",
			entityType:  "pages",
			installPath: "",
		},
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

	selectTemplateList := newSelectTemplateListModel(templates, itemDelegate{}, 0, 0)
	selectTemplateList.Title = "Select template:"
	selectTemplateList.SetShowStatusBar(false)
	selectTemplateList.SetFilteringEnabled(false)
	selectTemplateList.Styles.Title = titleStyle
	selectTemplateList.Styles.PaginationStyle = paginationStyle
	selectTemplateList.Styles.HelpStyle = helpStyle

	enterEntityInput := textinput.New()
	enterEntityInput.Placeholder = "app, dashboard, landing, etc"

	return model{
		state:          SELECT_TEMPLATE,
		selectTemplate: selectTemplate{list: selectTemplateList.Model},
		enterEntity:    enterEntity{input: enterEntityInput},
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.state {
	case SELECT_TEMPLATE:
		return selectTemplateUpdate(m, msg)
	case ENTER_ENTITY:
		return enterEntityInputUpdate(m, msg)
	default:
		return m, nil
	}
}

func (m model) View() string {
	switch m.state {
	case SELECT_TEMPLATE:
		return selectTemplateView(m)
	case ENTER_ENTITY:
		return enterEntityInputView(m)
	default:
		return "Unknown state: " + m.state
	}
}

func headerView() string {
	return theme().header.Render("Gas.dev")
}

func selectTemplateUpdate(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		listHeight := msg.Height - theme().header.GetHeight() - theme().header.GetVerticalMargins() - titleStyle.GetVerticalMargins()
		m.selectTemplate.list.SetSize(msg.Width, listHeight)
		return m, tea.ClearScreen
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, tea.Sequence(tea.ClearScreen, tea.Quit)
		case "enter":
			if selectedItem, ok := m.selectTemplate.list.SelectedItem().(item); ok {
				if selectedItem.entityGroup == "web" {
					m.state = ENTER_ENTITY
					return m, tea.Sequence(uicommon.NextState, m.enterEntity.input.Focus())
				}
			}
			// m.state = SELECT_ENTITY_GROUP
			return m, uicommon.NextState
		}
	}

	var cmd tea.Cmd
	m.selectTemplate.list, cmd = m.selectTemplate.list.Update(msg)
	return m, cmd
}

func selectTemplateView(m model) string {
	s := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Left, headerView(), lipgloss.NewStyle().Margin(1, 0, 0, 1).Render("Add resources - Pending installs (0)")),
		m.selectTemplate.list.View(),
	)
	return s
}

var (
	titleStyle        = lipgloss.NewStyle().MarginTop(1)
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
	paginationStyle   = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	helpStyle         = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
)

type selectTemplateListModel struct {
	list.Model
	cursor       int
	selectedId   string
	selectedItem item
}

func newSelectTemplateListModel(items []list.Item,
	delegate list.ItemDelegate,
	width int,
	height int) selectTemplateListModel {
	return selectTemplateListModel{
		Model:        list.New(items, delegate, width, height),
		cursor:       0,
		selectedId:   "",
		selectedItem: items[0].(item),
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
			m.selectedItem = m.Items()[m.cursor].(item)

		case "up", "k":
			m.cursor--
			if m.cursor < 0 {
				m.cursor = len(m.Items()) - 1
			}
			m.selectedItem = m.Items()[m.cursor].(item)
		}
	}

	var cmd tea.Cmd
	m.Model, cmd = m.Model.Update(msg)
	return m, cmd
}

func (m selectTemplateListModel) View() string {
	return m.Model.View()
}

func (l selectTemplateListModel) SelectedItem() item {
	return l.selectedItem
}

type item struct {
	id          string
	value       string
	entityGroup string
	entity      string
	entityType  string
	installPath string
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

func enterEntityInputUpdate(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, tea.Sequence(tea.ClearScreen, tea.Quit)
		case "enter":
			/*
				model.state = ADDED_TEMPLATE_CONFIRMED
				return model, uicommon.NextState
			*/
		}
	}

	var cmd tea.Cmd
	m.enterEntity.input, cmd = m.enterEntity.input.Update(msg)
	return m, cmd
}

func enterEntityInputView(m model) string {
	inputStyle := lipgloss.NewStyle().Margin(1, 0, 0, 1)

	inputView := fmt.Sprintf(
		"Selected \"%s\" template.\n\n",
		m.selectTemplate.list.SelectedItem().(item).value,
	)

	inputView += "Enter entity:\n\n"

	inputView += m.enterEntity.input.View()

	s := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Left, headerView(), lipgloss.NewStyle().Margin(1, 0, 0, 1).Render("Add resources - Pending installs (0)")),
		inputStyle.Render(inputView),
	)
	return s
}

/*
const (
	SELECT_TEMPLATE_LIST     = "SELECT_TEMPLATE_LIST"
	ENTER_ENTITY_GROUP_INPUT = "ENTER_ENTITY_GROUP_INPUT" // TODO: implement
	SELECT_ENTITY_GROUP_LIST = "SELECT_ENTITY_GROUP_LIST" // TODO: implement
	ENTER_ENTITY_INPUT       = "ENTER_ENTITY_INPUT"
	SELECT_ENTITY_LIST       = "SELECT_ENTITY_LIST" // TODO: implement
	ADDED_TEMPLATE_CONFIRMED = "ADDED_TEMPLATE_CONFIRMED"
	PENDING_INSTALLS         = "PENDING_INSTALLS"
)

var ui = uicommon.New()

type model struct {
	state                         string
	selectTemplateList            selectTemplateListModel
	entityInput                   textinput.Model
	addedTemplateConfirmedOptions uicommon.SelectModel
	pendingInstallsOptions        uicommon.SelectModel
}

func InitialModel() model {
	items := []list.Item{
		item{
			id:          "cloudflare-pages-remix-empty",
			value:       "Cloudflare Pages - Remix - Empty",
			entityGroup: "web",
			entity:      "",
			entityType:  "pages",
			installPath: "",
		},
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

	addTemplateConfirmOptions := uicommon.NewSelect()
	addTemplateConfirmOptions.Options = []uicommon.SelectOption{
		{Id: "pending-installs", Value: "Continue to pending installs"},
		{Id: "add-another", Value: "Add another"},
		{Id: "undo", Value: "Undo"},
	}
	addTemplateConfirmOptions.SelectedId = addTemplateConfirmOptions.Options[addTemplateConfirmOptions.Cursor].Id

	pendingInstallsOptions := uicommon.NewSelect()
	pendingInstallsOptions.Options = []uicommon.SelectOption{
		{Id: "install", Value: "Install"},
		{Id: "cancel", Value: "Cancel"},
	}
	pendingInstallsOptions.SelectedId = pendingInstallsOptions.Options[pendingInstallsOptions.Cursor].Id

	return model{
		state:                         SELECT_TEMPLATE_LIST,
		selectTemplateList:            selectTemplateList,
		entityInput:                   entityInput,
		addedTemplateConfirmedOptions: addTemplateConfirmOptions,
		pendingInstallsOptions:        pendingInstallsOptions,
	}
}

func (m model) Init() tea.Cmd {
	ui.Register(SELECT_TEMPLATE_LIST, uicommon.Fns{Update: selectTemplateListUpdate, View: selectTemplateListView})
	ui.Register(ENTER_ENTITY_INPUT, uicommon.Fns{Update: enterEntityInputUpdate, View: enterEntityInputView})
	ui.Register(ADDED_TEMPLATE_CONFIRMED, uicommon.Fns{Update: addedTemplateConfirmedUpdate, View: addedTemplateConfirmedView})
	ui.Register(PENDING_INSTALLS, uicommon.Fns{Update: pendingInstallsUpdate, View: pendingInstallsView})

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
		s := fmt.Sprintf("unknown state: %s\n\n", m.state)
		s += "Verify state, update, and view are registered\n\n"
		s += uicommon.EscView()
		return s
	}
	return uiFn.View(m)
}

func selectTemplateListUpdate(m tea.Model, msg tea.Msg) (tea.Model, tea.Cmd) {
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

func selectTemplateListView(m tea.Model) string {
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
	cursor       int
	selectedId   string
	selectedItem item
}

func newSelectTemplateListModel(items []list.Item,
	delegate list.ItemDelegate,
	width int,
	height int) selectTemplateListModel {
	return selectTemplateListModel{
		Model:        list.New(items, delegate, width, height),
		cursor:       0,
		selectedId:   "",
		selectedItem: items[0].(item),
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
			m.selectedItem = m.Items()[m.cursor].(item)

		case "up", "k":
			m.cursor--
			if m.cursor < 0 {
				m.cursor = len(m.Items()) - 1
			}
			m.selectedItem = m.Items()[m.cursor].(item)
		}
	}

	var cmd tea.Cmd
	m.Model, cmd = m.Model.Update(msg)
	return m, cmd
}

func (m selectTemplateListModel) View() string {
	return m.Model.View()
}

func (l selectTemplateListModel) SelectedItem() item {
	return l.selectedItem
}

type item struct {
	id          string
	value       string
	entityGroup string
	entity      string
	entityType  string
	installPath string
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
			model.state = ADDED_TEMPLATE_CONFIRMED
			model.selectTemplateList.selectedItem.entity = model.entityInput.Value()
			if model.selectTemplateList.selectedItem.entityGroup == "web" {
				model.selectTemplateList.selectedItem.installPath = fmt.Sprintf(
					"./gas/_%s-%s-%s",
					model.selectTemplateList.selectedItem.entityGroup,
					model.selectTemplateList.selectedItem.entity,
					model.selectTemplateList.selectedItem.entityType,
				)
			}
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

func addedTemplateConfirmedUpdate(m tea.Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	model := m.(model)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "enter" {
			if model.addedTemplateConfirmedOptions.SelectedId == "pending-installs" {
				model.state = PENDING_INSTALLS
				return model, uicommon.NextState
			}
		}
	}

	var cmd tea.Cmd
	model.addedTemplateConfirmedOptions, cmd = model.addedTemplateConfirmedOptions.Update(msg)
	return model, cmd
}

func addedTemplateConfirmedView(m tea.Model) string {
	model := m.(model)
	s := fmt.Sprintf(
		"Added \"%s\" template to pending installs.\n\n",
		model.selectTemplateList.SelectedItem().value,
	)
	s += fmt.Sprintf("Entity group: %s\n", model.selectTemplateList.SelectedItem().entityGroup)
	s += fmt.Sprintf("Entity: %s\n", model.selectTemplateList.SelectedItem().entity)
	s += fmt.Sprintf("Entity type: %s\n", model.selectTemplateList.SelectedItem().entityType)
	s += fmt.Sprintf("Download path: %s\n\n", model.selectTemplateList.SelectedItem().installPath)
	s += "What next?\n\n"
	s += fmt.Sprintf("%s\n\n", model.addedTemplateConfirmedOptions.View())
	s += uicommon.EscView()
	return s
}

func pendingInstallsUpdate(m tea.Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	model := m.(model)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "enter" {
			model.state = PENDING_INSTALLS
			return model, uicommon.NextState
		}
	}

	var cmd tea.Cmd
	model.pendingInstallsOptions, cmd = model.pendingInstallsOptions.Update(msg)
	return model, cmd
}

func pendingInstallsView(m tea.Model) string {
	model := m.(model)
	s := "Install templates?\n\n"
	s += fmt.Sprintf("%s\n\n", model.pendingInstallsOptions.View())
	s += uicommon.EscView()
	return s
}
*/
