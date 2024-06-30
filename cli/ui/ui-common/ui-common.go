package uicommon

import (
	tea "github.com/charmbracelet/bubbletea"
)

type NextStateType bool

/*
NextState() is a custom event that's used when transitioning
to the next state.

It's helpful for two reasons: 1) it makes the intended state
transition explicit to readers of the code, and 2) it gives the
next state's Update func a tea.Msg to hook into when the Update
func is first initiated. That hook is important for things like
emitting a spinner tea.Cmd on initialization if the state needs
to process something.

For example, if a state named "selectEmptyDirOption" gives the user
a choice of emptying a dir and the user selects "yes", the
"selectEmptyDirOption" Update func could set the model state to
"emptyingDir" and emit a NextState() tea.Msg. Then the "emptyingDir"
state could look for that tea.Msg and batch emit tea.Cmds for the
spinner and empty dir operation.

See the add and create commands for an example of this in action.
*/
func NextState() tea.Msg {
	return NextStateType(true)
}
