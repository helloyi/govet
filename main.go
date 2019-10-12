package main

import (
	"log"

	"github.com/helloyi/govet/cmd"
)

func main() {
	if err := cmd.Govet.Execute(); err != nil {
		log.Fatalln(err)
	}
}
