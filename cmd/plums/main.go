package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Ceinl/plums/internal/api"
)

var (
	version   = "0.1.0-dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	cfg := api.DefaultConfig()
	cfg.Version = version
	cfg.Commit = commit
	cfg.BuildDate = buildDate

	api.RegisterFlags(cfg)
	flag.Parse()

	if err := api.Run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "plums: %v\n", err)
		os.Exit(1)
	}
}
