package main

import (
	"os"

	"github.com/SPDG/unicli/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
