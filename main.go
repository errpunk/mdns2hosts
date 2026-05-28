package main

import (
	"os"

	"github.com/liutao/mdns2hosts/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
