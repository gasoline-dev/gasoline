package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add resources",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Add resources")
	},
}
