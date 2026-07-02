package main

import (
	plumsruntime "github.com/Ceinl/plums/plums/runtime"
)

var (
	version   = "0.1.0-dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	plumsruntime.Main(version, commit, buildDate)
}
