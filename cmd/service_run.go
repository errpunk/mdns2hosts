package cmd

import (
	"github.com/liutao/mdns2hosts/service"
	"github.com/spf13/cobra"
)

var serviceRunCmd = &cobra.Command{
	Use:    "service-run",
	Short:  "Run as a Windows service (called by SCM)",
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return service.RunService()
	},
}

func init() {
	rootCmd.AddCommand(serviceRunCmd)
}
