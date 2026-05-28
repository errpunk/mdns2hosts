package cmd

import (
	"fmt"

	"github.com/liutao/mdns2hosts/service"
	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall-service",
	Short: "Remove the mdns2hosts Windows service",
	Long:  `Stops and unregisters the mdns2hosts Windows service.`,
	Args:  cobra.NoArgs,
	RunE:  runUninstall,
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
}

func runUninstall(cmd *cobra.Command, args []string) error {
	if err := service.Uninstall(); err != nil {
		return fmt.Errorf("failed to uninstall service: %w", err)
	}
	fmt.Println("Service uninstalled.")
	return nil
}
