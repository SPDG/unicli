package main

import (
	"context"
	"log"
	"os"

	"github.com/SPDG/unicli/internal/mcpwrap"
)

func main() {
	if err := mcpwrap.RunStdio(context.Background()); err != nil {
		log.SetOutput(os.Stderr)
		log.Fatal(err)
	}
}
