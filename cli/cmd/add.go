package cmd

import (
	"fmt"
	"os"

	uiadd "gas/ui/ui-add"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add resources",
	Run: func(cmd *cobra.Command, args []string) {
		p := tea.NewProgram(uiadd.InitialModel(), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			fmt.Printf("Error: %v", err)
			os.Exit(1)
		}
	},
}
