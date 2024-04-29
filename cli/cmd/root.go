package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	configFile string
	rootCmd    = &cobra.Command{
		Use: "gas",
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

	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "config file (default is ./gas.config.json)")

	rootCmd.AddCommand(upCmd)
	rootCmd.AddCommand(newCmd)
}

func initConfig() {
	viper.SetDefault("resourceContainerDir", "gas")

	if configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		viper.SetConfigName("gas.config.json")
		viper.SetConfigType("json")
		viper.AddConfigPath(".")
	}

	err := viper.ReadInConfig()
	if err != nil {
		fmt.Printf("Error: unable to read config file: %s\n", err)
		os.Exit(1)
		return
	}

	godotenv.Load()

	viper.AutomaticEnv()

	requiredEnvVars := []string{"CLOUDFLARE_ACCOUNT_ID", "CLOUDFLARE_API_TOKEN"}
	err = ValidateRequiredEnvVars(requiredEnvVars)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
		return
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
