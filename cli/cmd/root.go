package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const customHelpTemplate = `{{with (or .Long .Short)}}{{. | trimTrailingWhitespaces}}

{{end}}CLI:
  {{.UseLine}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`

var (
	configFile string
	rootCmd    = &cobra.Command{
		Use:   "gas",
		Short: "gas is a CLI tool for managing your project",
		Long: `gas is a Command Line Interface (CLI) tool for managing your project.
It provides various commands to help you with project setup and management.

Custom Help:
  Environment Variables:
    CLOUDFLARE_ACCOUNT_ID  Your Cloudflare account ID
    CLOUDFLARE_API_TOKEN   Your Cloudflare API token

  Configuration:
    A config file named gas.config.json is required in the project root.`,
		Run: func(cmd *cobra.Command, args []string) {
			// If no subcommand is provided, run the 'add' command
			if len(args) == 0 {
				addCmd.Run(cmd, args)
			} else {
				cmd.Usage()
			}
		},
	}
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.SetHelpTemplate(customHelpTemplate)

	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "config file (default is ./gas.config.json)")

	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(upCmd)
}

func initConfig() {
	viper.SetDefault("resourceContainerDirPath", "gas")
	viper.SetDefault("upJsonPath", "gas.up.json")

	if configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		viper.SetConfigName("gas.config.json")
		viper.SetConfigType("json")
		viper.AddConfigPath(".")
	}

	// Check if there are any arguments and if the first one is not "create"
	if len(os.Args) > 1 && os.Args[1] != "create" {
		err := viper.ReadInConfig()
		if err != nil {
			fmt.Printf("Error: unable to read config file\n%s\n", err)
			os.Exit(1)
		}

		godotenv.Load()

		viper.AutomaticEnv()

		project := viper.GetString("project")
		if project == "" {
			fmt.Printf("Error: 'project' property is required in config file '%s'\n", viper.ConfigFileUsed())
			os.Exit(1)
		}

		requiredEnvVars := []string{"CLOUDFLARE_ACCOUNT_ID", "CLOUDFLARE_API_TOKEN"}
		err = ValidateRequiredEnvVars(requiredEnvVars)
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}
	}
}

func ValidateRequiredEnvVars(keys []string) error {
	var missingVars []string
	for _, key := range keys {
		if viper.GetString(key) == "" {
			missingVars = append(missingVars, key)
		}
	}
	if len(missingVars) > 0 {
		return fmt.Errorf("the following required environment variables are not set -> %s", strings.Join(missingVars, ", "))
	}
	return nil
}
