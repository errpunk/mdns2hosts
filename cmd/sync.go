package cmd

import (
	"fmt"
	"os"

	"github.com/liutao/mdns2hosts/hosts"
	"github.com/liutao/mdns2hosts/mdns"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync [name...]",
	Short: "Query mDNS names and sync their IPv4 addresses to the hosts file",
	Long: `Resolves one or more .local names via mDNS multicast and writes
the resulting IPv4 addresses into the managed block of the hosts file.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runSync,
}

func init() {
	rootCmd.AddCommand(syncCmd)
}

func runSync(cmd *cobra.Command, args []string) error {
	if err := hosts.EnsureBlock(); err != nil {
		return fmt.Errorf("failed to ensure hosts block: %w", err)
	}

	results, errs := mdns.ResolveAll(args)
	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", e)
	}

	if len(results) == 0 {
		return fmt.Errorf("no mDNS names could be resolved")
	}

	before, _, after, err := hosts.ReadHosts()
	if err != nil {
		return fmt.Errorf("failed to read hosts: %w", err)
	}

	if err := hosts.WriteHosts(before, results, after); err != nil {
		return fmt.Errorf("failed to write hosts: %w", err)
	}

	for name, ip := range results {
		fmt.Printf("%s -> %s\n", name, ip.String())
	}

	return nil
}
