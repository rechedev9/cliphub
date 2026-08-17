package main

import (
	"os"

	"github.com/rechedev9/cliphub/internal/streamcli"
)

func main() {
	os.Exit(streamcli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
