package uicommon

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type NextStateType bool

/*
NextState() is a custom tea.Msg that's used when transitioning
to the next state.

It's helpful for two reasons: 1) it makes the intended state
transition explicit to readers of the code, and 2) it gives the
next state's Update func a tea.Msg to hook into when the Update
func is first initiated. That hook is important for things like
emitting a spinner tea.Cmd on initialization if the state needs
to process something.
*/
func NextState() tea.Msg {
	return NextStateType(true)
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
	cursor     int
	selectedId string
	options    []option
}

type option struct {
	Id    string
	Value string
}

/*
NewSelect() is derived from:
https://github.com/charmbracelet/bubbletea/tree/4a9620e7134978771059ff7b481b6c9a8c611ac3/examples/result
*/
func NewSelect() SelectModel {
	return SelectModel{}
}

func (m SelectModel) init() tea.Cmd {
	return nil
}

func (m SelectModel) View() string {
	s := strings.Builder{}

	for i := 0; i < len(m.options); i++ {
		if m.cursor == i {
			s.WriteString("(•) ")
		} else {
			s.WriteString("( ) ")
		}
		s.WriteString(m.options[i].Value)
		if i < len(m.options)-1 {
			s.WriteString("\n")
		}
	}

	return s.String()
}

func (m SelectModel) Update(msg tea.Msg) (SelectModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "down", "j":
			m.cursor++
			if m.cursor >= len(m.options) {
				m.cursor = 0
			}
			m.selectedId = m.options[m.cursor].Id

		case "up", "k":
			m.cursor--
			if m.cursor < 0 {
				m.cursor = len(m.options) - 1
			}
			m.selectedId = m.options[m.cursor].Id

		case "tab":
			if m.cursor == len(m.options)-1 {
				m.cursor = 0
			} else {
				m.cursor++
			}
			m.selectedId = m.options[m.cursor].Id
		}
	}

	return m, nil
}

func (m *SelectModel) Reset() {
	m.cursor = 0
	if len(m.options) > 0 {
		m.selectedId = m.options[0].Id
	}
}
