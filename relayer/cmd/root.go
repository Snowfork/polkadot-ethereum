// Copyright 2020 Snowfork
// SPDX-License-Identifier: LGPL-3.0-only

package cmd

import (
	"fmt"
	"os"
	"path"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var dataDir string
var configFile string

var rootCmd = &cobra.Command{
	Use:          "artemis-relay",
	Short:        "Artemis Relay is a bridge between Ethereum and Polkadot",
	SilenceUsage: true,
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&dataDir, "data-dir", "", "data directory")
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "config file")

	rootCmd.AddCommand(runCmd())
	rootCmd.AddCommand(getBlockCmd())
	rootCmd.AddCommand(fetchMessagesCmd())
	rootCmd.AddCommand(subBeefyCmd())
}

func initConfig() {

	if configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println("Error: ", err)
			os.Exit(1)
		}

		viper.AddConfigPath(path.Join(home, ".config", "snowbridge-relay"))
		viper.AddConfigPath(".")

		viper.SetConfigName("config")
		viper.SetConfigType("toml")
	}

	viper.BindPFlag("global.data-dir", rootCmd.PersistentFlags().Lookup("data-dir"))

	if err := viper.ReadInConfig(); err != nil {
		fmt.Println("Error: ", err)
		os.Exit(1)
	}
}

// Execute adds all child commands to the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
