package main

import (
	"context"
	_ "embed"
	"os"

	"github.com/honganh1206/tinker/internal/cli"
)

func main() {
	root := cli.NewCLI()
	err := root.ExecuteContext(context.Background())
	if err != nil {
		os.Exit(1)
	}
}
