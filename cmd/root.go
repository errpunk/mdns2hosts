package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "mdns2hosts",
	Short: "Sync mDNS names to Windows hosts file",
	Long: `mdns2hosts queries mDNS (.local) names directly via multicast DNS
and writes resolved IPv4 addresses into the Windows hosts file
within a managed block, keeping other entries untouched.`,
	SilenceUsage: true,
}

func Execute() error {
	return rootCmd.Execute()
}
