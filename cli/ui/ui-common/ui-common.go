package uicommon

import (
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
