package cmd

import (
	"fmt"
	uicreate "gas/ui/ui-create"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create project",
	Run: func(cmd *cobra.Command, args []string) {
		p := tea.NewProgram(uicreate.InitialModel())
		if _, err := p.Run(); err != nil {
			fmt.Printf("Error: %v", err)
			os.Exit(1)
		}
	},
}
