package cmd

import (
	"fmt"
	"io"
	"net"
	"os"

	"github.com/liutao/mdns2hosts/hosts"
	"github.com/liutao/mdns2hosts/mdns"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync [name...]",
	Short: "Query mDNS names and sync their IPv4 addresses to the hosts file",
	Long: `Resolves one or more .local names via mDNS multicast and writes
the resulting IPv4 addresses into # mdns2hosts-tagged hosts entries.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runSync,
}

func init() {
	syncCmd.Flags().BoolVar(&syncDryRun, "dry-run", false, "print the updated hosts file content without writing it")
	rootCmd.AddCommand(syncCmd)
}

var (
	ensureHostsFile = hosts.EnsureBlock
	readHostsFile   = hosts.ReadHosts
	writeHostsFile  = hosts.WriteHosts
	renderHostsFile = hosts.RenderHosts
	resolveAllNames = mdns.ResolveAll
	syncDryRun      bool
)

func runSync(cmd *cobra.Command, args []string) error {
	if err := ensureHostsFile(); err != nil {
		return fmt.Errorf("failed to load hosts file: %w", err)
	}

	results, errs := resolveAllNames(args)
	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", e)
	}

	if len(results) == 0 {
		return fmt.Errorf("no mDNS names could be resolved")
	}

	before, _, after, err := readHostsFile()
	if err != nil {
		return fmt.Errorf("failed to read hosts: %w", err)
	}

	if syncDryRun {
		rendered, err := renderHostsFile(before, results, after)
		if err != nil {
			return fmt.Errorf("failed to render hosts: %w", err)
		}
		var out io.Writer = os.Stdout
		if cmd != nil {
			out = cmd.OutOrStdout()
		}
		fmt.Fprint(out, rendered)
		return nil
	}

	if err := writeHostsFile(before, results, after); err != nil {
		return fmt.Errorf("failed to write hosts: %w", err)
	}

	for name, ip := range results {
		fmt.Printf("%s -> %s\n", name, ip.String())
	}

	return nil
}

func cloneIPMap(in map[string]net.IP) map[string]net.IP {
	out := make(map[string]net.IP, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
