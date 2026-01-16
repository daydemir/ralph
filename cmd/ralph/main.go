package main

import (
	"os"

	"github.com/daydemir/ralph/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
