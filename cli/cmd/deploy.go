package cmd

import (
	"fmt"
	resources "gas/internal"

	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy resources to the cloud",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Deploying resources to the cloud")
		resourceContainerSubDirs, err := resources.GetContainerSubDirs("./gasoline")
		if err != nil {
			fmt.Printf("Error getting container subdirs: %s\n", err)
			return
		}
		fmt.Println("Resource container subdirs:", resourceContainerSubDirs)
	},
}
