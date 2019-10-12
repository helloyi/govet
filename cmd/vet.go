package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Govet govet command
var Govet = &cobra.Command{
	Use: "govet",
}

// init ...
func init() {
	Govet.AddCommand(checkCmd)
	Govet.PersistentFlags().Int("choke", 30, "maximum number of complaining message")
	if err := viper.BindPFlag("choke", Govet.PersistentFlags().Lookup("choke")); err != nil {
		panic(err)
	}
}
