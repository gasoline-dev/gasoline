package cmd

import (
	"fmt"
	"os"

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

	rootCmd.AddCommand(deployCmd)
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

	viper.AutomaticEnv()

	viper.ReadInConfig()

	/*
		err := viper.ReadInConfig()
		if err != nil {
			fmt.Printf("Error: unable to read config file: %s\n", err)
			os.Exit(1)
		}
	*/
}
