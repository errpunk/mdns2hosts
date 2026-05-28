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

	executeCommand = cmd.Execute
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	for _, arg := range args {
		if arg == "-v" || arg == "--version" {
			fmt.Printf("mdns2hosts %s commit=%s built=%s\n", Version, Commit, BuildDate)
			return 0
		}
	}

	if err := executeCommand(); err != nil {
		return 1
	}
	return 0
}
