package uiadd

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

type errMsg error

type model struct {
	textarea textarea.Model
	err      error
}

func InitialModel() model {
	ti := textarea.New()
	ti.Placeholder = "Once upon a time..."
	ti.Focus()

	return model{
		textarea: ti,
		err:      nil,
	}
}

func (m model) Init() tea.Cmd {
	return textarea.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			if m.textarea.Focused() {
				m.textarea.Blur()
			}
		case tea.KeyCtrlC:
			return m, tea.Quit
		default:
			if !m.textarea.Focused() {
				cmd = m.textarea.Focus()
				cmds = append(cmds, cmd)
			}
		}

	// We handle errors just like any other message
	case errMsg:
		m.err = msg
		return m, nil
	}

	m.textarea, cmd = m.textarea.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	return fmt.Sprintf(
		"Tell me a story.\n\n%s\n\n%s",
		m.textarea.View(),
		"(ctrl+c to quit)",
	) + "\n\n"
}

/*
type model struct {
	screen         screen
	tab            tab
	terminalHeight int
	terminalWidth  int
	showModal      bool
	modal          modalModel
}

var navStyle = lipgloss.NewStyle().Height(1).Margin(0, 0, 1, 0)

var navLinkActiveStyle = lipgloss.NewStyle().Underline(true)

var screenStyle = lipgloss.NewStyle().Padding(1, 1, 1, 1)

var titleStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("8")).
	Height(1).
	Width(11).
	AlignHorizontal(lipgloss.Center).
	Bold(true).
	Margin(0, 1, 1, 0)

var titleMetaStyle = lipgloss.NewStyle().Height(1).Margin(0, 0, 0, 0)

const (
	MAIN screen = iota
)

type screen int

var screens = uicommon.New[model]()

const (
	MAIN_SELECT_TEMPLATE tab = iota
	MAIN_PENDING_INSTALLS
)

type tab = int

var tabs = uicommon.New[model]()

func InitialModel() model {
	screens.Register(int(MAIN), uicommon.Fns[model]{
		Update: mainUpdate,
		View:   mainView,
	})

	tabs.Register(int(MAIN_SELECT_TEMPLATE), uicommon.Fns[model]{
		Update: mainSelectTemplateUpdate,
		View:   mainSelectTemplateView,
	})

	tabs.Register(int(MAIN_PENDING_INSTALLS), uicommon.Fns[model]{
		Update: mainPendingInstallsUpdate,
		View:   mainPendingInstallsView,
	})

	return model{
		screen:    MAIN,
		showModal: true,
		modal: modalModel{
			title:   "Example Modal",
			content: "This is a modal overlay.\nPress 'm' to close.",
		},
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.terminalHeight = msg.Height
		m.terminalWidth = msg.Width
		return m, uicommon.Tx
	case tea.KeyMsg:
		switch msg.String() {
		case "m":
			// Toggle modal visibility
			m.showModal = !m.showModal
			return m, nil
		case "q":
			return m, tea.Quit
		case "tab":
			m.nextTab()
			return m, nil
		case "shift+tab":
			m.prevTab()
			return m, nil
		}
	}

	screenFn, ok := screens.Fns[int(m.screen)]
	if !ok {
		return m, nil
	}
	return screenFn.Update(m, msg)
}

func (m model) View() string {
	// Get the main content
	content := m.getMainContent()

	// If modal is visible, overlay it on top of the main content
	if m.showModal {
		return lipgloss.Place(
			m.terminalWidth,
			m.terminalHeight,
			lipgloss.Center,
			lipgloss.Center,
			m.modal.View(),
			lipgloss.WithWhitespaceChars("!"),
			lipgloss.WithWhitespaceForeground(lipgloss.Color("240")),
		)
	}

	return content
}

func (m model) getMainContent() string {
	tabFn, ok := tabs.Fns[int(m.tab)]
	if !ok {
		return "Unknown tab"
	}
	return tabFn.View(m)
}

func (m *model) nextTab() {
	m.tab = (m.tab + 1) % tabs.Count()
}

func (m *model) prevTab() {
	m.tab = (m.tab - 1 + tabs.Count()) % tabs.Count()
}

func mainUpdate(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "tab":
			m.nextTab()
			return m, nil
		case "shift+tab":
			m.prevTab()
			return m, nil
		}
	}

	tabFn, ok := tabs.Fns[int(m.tab)]
	if !ok {
		return m, nil
	}
	return tabFn.Update(m, msg)
}

func mainView(m model) string {
	tabFn, ok := tabs.Fns[int(m.tab)]
	if !ok {
		return "Unknown tab"
	}
	return tabFn.View(m)
}

func headerView() string {
	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		titleStyle.Render("Gas.dev"),
		titleMetaStyle.Render("Add resources"),
	)
}

type navLinksType []navLink

type navLink struct {
	id   tab
	text string
}

func navView(t tab) string {
	navLinks := navLinksType{
		{id: MAIN_SELECT_TEMPLATE, text: "Templates"},
		{id: MAIN_PENDING_INSTALLS, text: "Pending installs (0)"},
	}

	s := ""
	navLinkCount := len(navLinks)
	for i, link := range navLinks {
		if link.id == t {
			s += navLinkActiveStyle.Render(link.text)
		} else {
			s += link.text
		}

		if i < navLinkCount-1 {
			s += " • "
		}
	}

	return navStyle.Render(s)
}

func mainSelectTemplateUpdate(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Making sure this bubbles up
		if msg.String() == "esc" {
			return m, tea.Quit
		}
	}

	return m, nil
}

var asciiArtStyle = lipgloss.NewStyle().
	Margin(1, 0, 1, 0)

func colorize(text, color string) string {
	return fmt.Sprintf("\x1b[%sm%s\x1b[0m", color, text)
}

func mainSelectTemplateView(m model) string {
	asciiArt := `
		:..,
            ~,   &~!------!~!!
             ~ !~~~~      *~~!~!
             !~~~~~~~~~~~~~......
           ;;~~~---------~~~=!~~~~
           !:~~~~------!-~~!:~~--~
           !:~~~----~--~-~~!:!  ,;
           !:~~~---------~~~;!  ,;
           !:~~~--!--~-----~;~  ,;
           ~;~~-----------~~!~~~!~
           ~;~~~------------!~~~~~
           !:!~~------------~~~~~
            ~----------------*
`

	lines := strings.Split(asciiArt, "\n")
	coloredLines := make([]string, len(lines))

	for i, line := range lines {
		switch {
		case i < 4:
			coloredLines[i] = colorize(line, "37") // Red
		case i < 7:
			coloredLines[i] = colorize(line, "91") // Light Red
		case i < 10:
			coloredLines[i] = colorize(line, "31") // Yellow
		default:
			coloredLines[i] = colorize(line, "38;5;208") // Orange
		}
	}

	gradientAsciiArt := strings.Join(coloredLines, "\n")

	s := lipgloss.JoinVertical(
		lipgloss.Left,
		headerView(),
		navView(MAIN_SELECT_TEMPLATE),
		asciiArtStyle.Render(gradientAsciiArt),
		fmt.Sprintf("Select template (q to quit) %d", m.screen),
	)
	return screenStyle.Render(s)
}

func mainPendingInstallsUpdate(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func mainPendingInstallsView(m model) string {
	s := lipgloss.JoinVertical(
		lipgloss.Left,
		headerView(),
		navView(MAIN_PENDING_INSTALLS),
		"Pending installs (q to quit)",
	)
	return screenStyle.Render(s)
}

// Modal component
var modalStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("62")).
	Padding(1, 0).
	Width(60).
	Height(10)

type modalModel struct {
	title   string
	content string
}

func (m modalModel) View() string {
	return modalStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Center,
			lipgloss.NewStyle().Bold(true).Render(m.title),
			"",
			m.content,
		),
	)
}
*/

/*
const tabCount = 3

var terminalStyle = lipgloss.NewStyle().Padding(1, 1, 1, 1)

var contentStyle = lipgloss.NewStyle().Margin(0, 0, 0, 0)

var enterEntityHelpShortKeyStyle = lipgloss.NewStyle().Height(1)

var navStyle = lipgloss.NewStyle().Height(1).Margin(0, 0, 1, 0)
var navLinkActiveStyle = lipgloss.NewStyle().Underline(true)

var pStyle = lipgloss.NewStyle()

var titleStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("8")).
	Height(1).
	Width(11).
	AlignHorizontal(lipgloss.Center).
	Bold(true).
	Margin(0, 1, 1, 0)

var titleMetaStyle = lipgloss.NewStyle().Height(1).Margin(0, 0, 0, 0)

const (
	SELECT_TEMPLATE state = iota
	ENTER_ENTITY
	PENDING_INSTALLS
)

type state int

type model struct {
	state          state
	terminalHeight int
	terminalWidth  int
	selectTemplate selectTemplate
	enterEntity    enterEntity
}

type selectTemplate struct {
	list list.Model
}

type enterEntity struct {
	help  help.Model
	input textinput.Model
	keys  enterEntityKeyMap
}

func InitialModel() model {
	templates := []list.Item{
		item{
			id:          "cloudflare-pages-remix",
			value:       "Cloudflare Pages - Remix",
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
	selectTemplateList.Styles.Title = listTitleStyle
	selectTemplateList.Styles.TitleBar = lipgloss.NewStyle()
	selectTemplateList.Styles.PaginationStyle = paginationStyle
	selectTemplateList.Styles.HelpStyle = helpStyle

	enterEntityInput := textinput.New()
	enterEntityInput.Placeholder = "app, dashboard, landing, etc"

	enterEntityHelp := help.New()
	enterEntityHelp.Styles = help.Styles{
		ShortKey: enterEntityHelpShortKeyStyle,
	}

	return model{
		state:          SELECT_TEMPLATE,
		selectTemplate: selectTemplate{list: selectTemplateList.Model},
		enterEntity: enterEntity{
			help:  enterEntityHelp,
			input: enterEntityInput,
			keys:  enterEntityKeys,
		},
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.terminalHeight = msg.Height
		m.terminalWidth = msg.Width
		return m, uicommon.Tx
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			m.nextTab()
		case "shift+tab":
			m.prevTab()
		}
	}

	switch m.state {
	case SELECT_TEMPLATE:
		return selectTemplateUpdate(m, msg)
	case ENTER_ENTITY:
		return enterEntityUpdate(m, msg)
	case PENDING_INSTALLS:
		return pendingInstallsUpdate(m, msg)
	default:
		return m, nil
	}
}

func (m model) View() string {
	switch m.state {
	case SELECT_TEMPLATE:
		return selectTemplateView(m)
	case ENTER_ENTITY:
		return enterEntityView(m)
	case PENDING_INSTALLS:
		return pendingInstallsView(m)
	default:
		return "Unknown state"
	}
}

func (m *model) nextTab() {
	m.state = (m.state + 1) % tabCount
}

func (m *model) prevTab() {
	m.state = (m.state - 1 + tabCount) % tabCount
}

func headerView() string {
	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		titleStyle.Render("Gas.dev"),
		titleMetaStyle.Render("Add resources"),
	)
}

type navLinksType []navLink

type navLink struct {
	id   state
	text string
}

func navView(currState state) string {
	navLinks := navLinksType{
		{id: SELECT_TEMPLATE, text: "Templates"},
		{id: PENDING_INSTALLS, text: "Pending installs (0)"},
	}

	s := ""
	navLinkCount := len(navLinks)
	for i, link := range navLinks {
		if link.id == currState {
			s += navLinkActiveStyle.Render(link.text)
		} else {
			s += link.text
		}

		if i < navLinkCount-1 {
			s += " • "
		}
	}

	return navStyle.Render(s)
}

func selectTemplateUpdate(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case uicommon.TxMsg:
		listHeight := m.terminalHeight -
			terminalStyle.GetVerticalPadding() -
			titleStyle.GetHeight() -
			titleStyle.GetVerticalMargins() -
			navStyle.GetHeight() -
			navStyle.GetVerticalMargins()
		m.selectTemplate.list.SetSize(m.terminalWidth, listHeight)
		return m, tea.ClearScreen
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, tea.Sequence(tea.ClearScreen, tea.Quit)
		case "enter":
			if selectedItem, ok := m.selectTemplate.list.SelectedItem().(item); ok {
				if selectedItem.entityGroup == "web" {
					m.state = ENTER_ENTITY
					return m, tea.Sequence(uicommon.Tx, m.enterEntity.input.Focus())
				}
			}
			// m.state = SELECT_ENTITY_GROUP
			return m, uicommon.Tx
		}
	}

	var cmd tea.Cmd
	m.selectTemplate.list, cmd = m.selectTemplate.list.Update(msg)
	return m, cmd
}

func selectTemplateView(m model) string {
	s := lipgloss.JoinVertical(lipgloss.Left,
		headerView(),
		navView(m.state),
		m.selectTemplate.list.View(),
	)
	return terminalStyle.Render(s)
}

var (
	listTitleStyle    = lipgloss.NewStyle().Margin(0, 0, 0, 0).Padding(0, 0, 0, 0)
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
	paginationStyle   = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	helpStyle         = list.DefaultStyles().HelpStyle.PaddingLeft(0)
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

func enterEntityUpdate(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case uicommon.TxMsg:
		contentHeight := m.terminalHeight -
			terminalStyle.GetVerticalPadding() -
			titleStyle.GetHeight() -
			titleStyle.GetVerticalMargins() -
			enterEntityHelpShortKeyStyle.GetHeight()
		contentStyle = contentStyle.Height(contentHeight)
		return m, tea.ClearScreen
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, tea.Sequence(tea.ClearScreen, tea.Quit)
		case "enter":
				//model.state = ADDED_TEMPLATE_CONFIRMED
				//return model, uicommon.NextState
		}
	}

	var cmd tea.Cmd
	m.enterEntity.input, cmd = m.enterEntity.input.Update(msg)
	return m, cmd
}

func enterEntityView(m model) string {
	inputView := fmt.Sprintf(
		"Selected \"%s\" template.\n\n",
		m.selectTemplate.list.SelectedItem().(item).value,
	)

	inputView += "Enter entity:\n\n"

	inputView += m.enterEntity.input.View()

	helpView := m.enterEntity.help.View(m.enterEntity.keys)

	s := lipgloss.JoinVertical(lipgloss.Left,
		headerView(),
		contentStyle.Render(inputView),
		helpView,
	)
	return terminalStyle.Render(s)
}

type enterEntityKeyMap struct {
	Enter key.Binding
	Esc   key.Binding
}

var enterEntityKeys = enterEntityKeyMap{
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "submit"),
	),
	Esc: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "cancel"),
	),
}

func (k enterEntityKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Enter, k.Esc}
}

func (k enterEntityKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Enter},
		{k.Esc},
	}
}

func pendingInstallsUpdate(m tea.Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func pendingInstallsView(m tea.Model) string {
	return "Pending installs"
}


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
