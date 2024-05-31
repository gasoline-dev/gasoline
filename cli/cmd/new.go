package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var newCmd = &cobra.Command{
	Use:   "new [resource]",
	Short: "Add new resource",
	//Args:  cobra.ExactArgs(1),
	Args: func(cmd *cobra.Command, args []string) error {
		// Optionally run one of the validators provided by cobra
		/*
			if err := cobra.MinimumNArgs(1)(cmd, args); err != nil {
				return err
			}
		*/
		return fmt.Errorf("invalid color specified: %s", args[0])
	},
	Run: func(cmd *cobra.Command, args []string) {
		//path := args[0]
		//fmt.Printf("Adding resource for path: %s\n", path)
	},
	Example: `  gas new cloudflare:worker:api:hono`,
}
