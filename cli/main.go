package main

import (
	"os"

	"github.com/HafizalJohari/eyeVesa-community/cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
