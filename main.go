package main

import (
	"fmt"
	"os"

	"github.com/liutao/mdns2hosts/cmd"
)

// Set via ldflags at build time.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "-v" || arg == "--version" {
			fmt.Printf("mdns2hosts %s commit=%s built=%s\n", Version, Commit, BuildDate)
			return
		}
	}

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
