package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	svcNames    []string
	svcInterval string
)

var installCmd = &cobra.Command{
	Use:   "install-service",
	Short: "Install mdns2hosts as a Windows service",
	Long: `Registers mdns2hosts as a Windows service that periodically
syncs the configured mDNS names to the hosts file. Requires Administrator privileges.`,
	Args: cobra.NoArgs,
	RunE: runInstall,
}

func init() {
	installCmd.Flags().StringSliceVarP(&svcNames, "name", "n", nil, "mDNS names to sync (comma-separated)")
	installCmd.Flags().StringVarP(&svcInterval, "interval", "i", "30s", "Sync interval")
	installCmd.MarkFlagRequired("name")
	rootCmd.AddCommand(installCmd)
}

func runInstall(cmd *cobra.Command, args []string) error {
	exePath, err := serviceExePath()
	if err != nil {
		return fmt.Errorf("cannot determine executable path: %w", err)
	}

	if err := installService(svcNames, svcInterval, exePath); err != nil {
		return fmt.Errorf("failed to install service: %w", err)
	}

	fmt.Printf("Service installed. It will sync %v every %s.\n", svcNames, svcInterval)
	fmt.Println("Start it with: sc start mdns2hosts")
	return nil
}
