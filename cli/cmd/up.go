package cmd

import (
	"fmt"
	"gas/resources"
	"os"

	"github.com/spf13/cobra"
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Deploy resources",
	Run: func(cmd *cobra.Command, args []string) {
		r := resources.New()

		err := r.InitWithUp()
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}

		if r.HasNamesToDeploy() {
			err = r.Deploy()
			if err != nil {
				fmt.Println("Error:", err)
				os.Exit(1)
			}
		}

		fmt.Println("No resource changes to deploy")

		os.Exit(0)
	},
}
