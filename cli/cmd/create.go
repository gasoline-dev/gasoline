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
			fmt.Printf("Error: %v", err)
			os.Exit(1)
		}
	},
}

type model struct {
	state    state
	dirInput textinput.Model
	err      error
}

type errMsg struct{ err error }

func (e errMsg) Error() string { return e.err.Error() }

type state string

const (
	showDirInputView state = "showDirInputView"
)

func initialModel() model {
	dirInput := textinput.New()
	dirInput.Placeholder = "./example"
	dirInput.Focus()

	return model{
		state:    showDirInputView,
		dirInput: dirInput,
		err:      nil,
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) View() string {
	if m.err != nil {
		s := fmt.Sprintf("Error: %s\n\n", m.err)
		s += help()
		return s
	}

	if m.state == showDirInputView {
		return dirInputView(m)
	}

	return ""
}

func dirInputView(m model) string {
	s := fmt.Sprintf(
		"Directory path:\n\n%s\n\n",
		m.dirInput.View())
	s += help()
	return s
}

func help() string {
	return "Press esc or q to exit\n"
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		k := msg.String()
		if k == "esc" || k == "q" {
			return m, tea.Quit
		}
	}

	if m.state == showDirInputView {
		return dirInputUpdate(m, msg)
	}

	return m, nil
}

func dirInputUpdate(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd

	m.dirInput, cmd = m.dirInput.Update(msg)

	return m, cmd
}
