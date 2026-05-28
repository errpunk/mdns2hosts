package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove mdns2hosts-managed entries from the hosts file",
	Long:  `Removes all hosts entries tagged with # mdns2hosts.`,
	Args:  cobra.NoArgs,
	RunE:  runClean,
}

func init() {
	rootCmd.AddCommand(cleanCmd)
}

func runClean(cmd *cobra.Command, args []string) error {
	if err := cleanHostsFile(); err != nil {
		return fmt.Errorf("failed to clean managed hosts entries: %w", err)
	}
	fmt.Println("Managed hosts entries removed.")
	return nil
}
