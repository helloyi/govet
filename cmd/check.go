package cmd

import (
	"log"

	"github.com/spf13/cobra"

	"github.com/helloyi/govet/config"
	"github.com/helloyi/govet/vet"
)

// checkCmd check command
var checkCmd = &cobra.Command{
	Use:     "check",
	Short:   "Run check on the project",
	Example: "govet check",
	Run:     run,
}

func run(cmd *cobra.Command, args []string) {
	c, err := config.New()
	if err != nil {
		log.Fatalln(err)
	}

	v, err := vet.New(c)
	if err != nil {
		log.Fatalln(err)
	}

	v.Do()
}
