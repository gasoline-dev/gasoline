package cmd

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create project",
	Run: func(cmd *cobra.Command, args []string) {
		p := tea.NewProgram(initialModel(), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			fmt.Printf("Alas, there's been an error: %v", err)
			os.Exit(1)
		}
	},
}

type model struct {
	state state
	err   error
}

type state struct {
	value stateValue
	data  stateData
}

type stateValue string

type stateData struct {
	another textinput.Model
	dir     textinput.Model
	ready   bool
	width   int
}

type (
	errMsg error
)

const (
	showingDirInput     stateValue = "showingDirInput"
	showingAnotherInput stateValue = "showingAnotherInput"
)

func initialModel() model {
	dirInput := textinput.New()
	dirInput.Placeholder = "./example"
	dirInput.Focus()
	dirInput.CharLimit = 156
	dirInput.Width = 20

	anotherInput := textinput.New()
	anotherInput.Placeholder = "./another"
	anotherInput.CharLimit = 156
	anotherInput.Width = 20

	return model{
		state: state{
			value: showingDirInput,
			data: stateData{
				another: anotherInput,
				dir:     dirInput,
				ready:   false,
				width:   0,
			},
		},
		err: nil,
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) View() string {
	if m.state.value == showingDirInput {
		return showingInputDirView(m)
	} else {
		return showingAnotherInputView(m)
	}
}

func showingInputDirView(m model) string {
	return fmt.Sprintf(
		"Directory path:\n\n%s\n\n%s",
		m.state.data.dir.View(),
		"(esc to quit)",
	) + "\n"
}

func showingAnotherInputView(m model) string {
	return fmt.Sprintf(
		"Directory path!!:\n\n%s\n\n%s",
		m.state.data.another.View(),
		"(esc to quit)",
	) + "\n"
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		k := msg.String()
		if k == "q" || k == "esc" || k == "ctrl+c" {
			return m, tea.Quit
		}
	}

	// err should be here?

	if m.state.value == showingDirInput {
		return showingInputDirUpdater(m, msg)
	} else {
		return showingAnotherInputUpdater(m, msg)
	}

	return m, nil // should be a default view (an impossible state?)
}

func showingInputDirUpdater(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			m.state.data.dir.Blur()
			m.state.value = showingAnotherInput
			return m, m.state.data.another.Focus()
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		}
	case errMsg:
		m.err = msg //? don't know what to do with err
		return m, nil
	}

	var cmd tea.Cmd

	m.state.data.dir, cmd = m.state.data.dir.Update(msg)

	return m, cmd
}

func showingAnotherInputUpdater(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			return m, nil
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		}
	case errMsg:
		m.err = msg //? don't know what to do with err
		return m, nil
	}

	var cmd tea.Cmd

	m.state.data.another, cmd = m.state.data.another.Update(msg)

	return m, cmd
}
