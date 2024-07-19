package uicommon

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

/*
uiCommon contains a map of UI states and their corresponding
update and view functions.
*/
type uiCommon struct {
	Fns map[string]Fns
}

type Fns struct {
	Update updateFn
	View   viewFn
}

type (
	updateFn func(m tea.Model, msg tea.Msg) (tea.Model, tea.Cmd)
	viewFn   func(m tea.Model) string
)

func New() *uiCommon {
	return &uiCommon{
		Fns: make(map[string]Fns),
	}
}

func (u *uiCommon) Register(state string, fns Fns) {
	u.Fns[state] = fns
}

/*
HandleMsgs() is a helper function for handling messages
that are common to all states.
*/
func HandleMsgs(msg tea.Msg, state string) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return tea.ClearScreen
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			// User shouldn't be able to exit if state is doing IO.
			if !strings.Contains(state, "DOWNLOADING") &&
				!strings.Contains(state, "INSTALLING") &&
				!strings.Contains(state, "PROCESSING") {
				return tea.Sequence(tea.ClearScreen, tea.Quit)
			}
		}
	}
	return nil
}

/*
EscView() is a helper function for displaying a message
to the user to press esc to exit the program. The esc
cmd is handled in HandleMsgs().
*/
func EscView() string {
	return "Press esc to exit\n\n"
}

type TxMsg bool

/*
Tx() (transmit) is a custom tea.Msg used for transitioning views
to the next state and/or forcing redraws of views after resizing.

It's helpful for three reasons: 1) it makes the intended state
transition explicit to readers of the code, 2) it gives the next
state's Update func a tea.Msg to hook into when that Update func
is first initiated. The hook is important for things like emitting
a spinner tea.Cmd on initialization if the state needs to process
something. 3) the root Update func can listen for tea.WindowSizeMsg
and emit Tx, which the active state Update func will receive, forcing
a redraw of that state's view.
*/
func Tx() tea.Msg {
	return TxMsg(true)
}

type FinalStateType bool

/*
FinalState() is a custom tea.Msg that's used when transitioning
to the final state.

It's helpful for two reasons: 1) it makes the intended state
transition explicit to readers of the code, and 2) it gives the
model's Update func a tea.Msg to hook into when the program's
states have finished processing and the program is about to
shut down. That is important for things like emitting tea.Cmds
for clearing the screen and quitting the program.
*/
func FinalState() tea.Msg {
	return FinalStateType(true)
}

type InputErr struct {
	Msg string
}

/*
InputErr is a custom error type for setting errors on bubbletea's
textinput.

textinput has a Validate property:
https://github.com/charmbracelet/bubbles/blob/64a67d167062e075d80a132afc0851fd1b2c6b89/textinput/textinput.go#L142

An example implementation can be seen here:
https://github.com/charmbracelet/bubbletea/blob/ab280576a5c4c8f8da4bf1cc97f3bde214cdef63/examples/credit-card-form/main.go#L106

However, it's easier to set errors directly in Update funcs. It
makes the error control explicit to the reader.

So textinput's Model has an Err field of type error:
https://github.com/charmbracelet/bubbles/blob/64a67d167062e075d80a132afc0851fd1b2c6b89/textinput/textinput.go#L87

That means Update funcs can set errors directly like this:

	m.ctx.dirPathInput.Err = &inputErr{
		Msg: "Directory path is required",
	}

And View funcs can set and display errors like this:

	if m.ctx.dirPathInput.Err != nil {
		var inputErr *inputErr
		switch {
		case errors.As(m.ctx.dirPathInput.Err, &inputErr):
			s += fmt.Sprintf("%v\n\n", m.ctx.dirPathInput.Err)
		default:
			s += fmt.Sprintf("Error: %v\n\n", m.ctx.dirPathInput.Err)
		}
	}

There's a good write-up on extending Go's error interface here:
https://earthly.dev/blog/golang-errors/
*/
func (e *InputErr) Error() string {
	return e.Msg
}

type SelectModel struct {
	Cursor     int
	SelectedId string
	Options    []SelectOption
}

type SelectOption struct {
	Id    string
	Value string
}

/*
NewSelect() is derived from:
https://github.com/charmbracelet/bubbletea/tree/4a9620e7134978771059ff7b481b6c9a8c611ac3/examples/result

It's a simple select UI for choosing between a small list of options.
*/
func NewSelect() SelectModel {
	return SelectModel{}
}

func (m SelectModel) init() tea.Cmd {
	return nil
}

func (m SelectModel) Update(msg tea.Msg) (SelectModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "down", "j":
			m.Cursor++
			if m.Cursor >= len(m.Options) {
				m.Cursor = 0
			}
			m.SelectedId = m.Options[m.Cursor].Id

		case "up", "k":
			m.Cursor--
			if m.Cursor < 0 {
				m.Cursor = len(m.Options) - 1
			}
			m.SelectedId = m.Options[m.Cursor].Id

		case "tab":
			if m.Cursor == len(m.Options)-1 {
				m.Cursor = 0
			} else {
				m.Cursor++
			}
			m.SelectedId = m.Options[m.Cursor].Id
		}
	}

	return m, nil
}

func (m SelectModel) View() string {
	s := strings.Builder{}

	for i := 0; i < len(m.Options); i++ {
		if m.Cursor == i {
			s.WriteString("(•) ")
		} else {
			s.WriteString("( ) ")
		}
		s.WriteString(m.Options[i].Value)
		if i < len(m.Options)-1 {
			s.WriteString("\n")
		}
	}

	return s.String()
}

func (m *SelectModel) Reset() {
	m.Cursor = 0
	if len(m.Options) > 0 {
		m.SelectedId = m.Options[0].Id
	}
}
