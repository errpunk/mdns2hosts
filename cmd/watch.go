package cmd

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"time"

	"github.com/spf13/cobra"
)

var watchInterval time.Duration

var watchCmd = &cobra.Command{
	Use:   "watch [name...]",
	Short: "Continuously monitor mDNS names and update hosts on IP changes",
	Long: `Polls mDNS names at the configured interval and updates the hosts
file whenever an IP address changes. Runs until interrupted.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runWatch,
}

func init() {
	watchCmd.Flags().DurationVarP(&watchInterval, "interval", "i", 30*time.Second, "Polling interval")
	rootCmd.AddCommand(watchCmd)
}

func runWatch(cmd *cobra.Command, args []string) error {
	return runWatchWithStop(args, nil)
}

func runWatchWithStop(args []string, stop <-chan struct{}) error {
	if err := ensureHostsFile(); err != nil {
		return fmt.Errorf("failed to load hosts file: %w", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	ticker := time.NewTicker(watchInterval)
	defer ticker.Stop()

	fmt.Printf("Watching %d names every %s...\n", len(args), watchInterval)

	// Do an immediate first sync
	lastIPs := syncOnce(args)

	for {
		select {
		case <-stop:
			fmt.Println("\nStopped watching.")
			return nil
		case <-sigCh:
			fmt.Println("\nStopped watching.")
			return nil
		case <-ticker.C:
			currentIPs := syncOnce(args)
			if ipsChanged(lastIPs, currentIPs) {
				fmt.Println("IP changes detected, updating hosts...")
				writeHosts(currentIPs)
				lastIPs = currentIPs
			}
		}
	}
}

func syncOnce(names []string) map[string]net.IP {
	results, errs := resolveAllNames(names)
	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", e)
	}
	return results
}

func ipsChanged(a, b map[string]net.IP) bool {
	if len(a) != len(b) {
		return true
	}
	for k, v := range a {
		if !v.Equal(b[k]) {
			return true
		}
	}
	return false
}

func writeHosts(entries map[string]net.IP) {
	before, _, after, err := readHostsFile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading hosts: %v\n", err)
		return
	}
	if err := writeHostsFile(before, entries, after); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing hosts: %v\n", err)
	}
}
