package cmd

import (
	"fmt"

	"github.com/liutao/mdns2hosts/hosts"
	"github.com/spf13/cobra"
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove the mdns2hosts managed block from the hosts file",
	Long:  `Removes all entries between the # BEGIN mdns2hosts and # END mdns2hosts markers.`,
	Args:  cobra.NoArgs,
	RunE:  runClean,
}

func init() {
	rootCmd.AddCommand(cleanCmd)
}

func runClean(cmd *cobra.Command, args []string) error {
	if err := hosts.CleanBlock(); err != nil {
		return fmt.Errorf("failed to clean hosts block: %w", err)
	}
	fmt.Println("Managed hosts block removed.")
	return nil
}
