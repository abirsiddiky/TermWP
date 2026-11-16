package cli

import (
	"strings"

	"github.com/spf13/cobra"
	"termwp/internal/system"
)

var hostsCmd = &cobra.Command{
	Use:    "hosts",
	Short:  "Internal helper for hosts file manipulation with elevation",
	Hidden: true, // Internal helper command
}

var hostsAddCmd = &cobra.Command{
	Use:    "add <domain> [ip]",
	Short:  "Add domain to system hosts file",
	Hidden: true,
	Args:   cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		domain := strings.ToLower(args[0])
		ip := "127.0.0.1"
		if len(args) > 1 {
			ip = args[1]
		}
		return system.ReadAndModifyHosts(func(domains map[string]string) {
			domains[domain] = ip
		})
	},
}

var hostsRemoveCmd = &cobra.Command{
	Use:    "remove <domain>",
	Short:  "Remove domain from system hosts file",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		domain := strings.ToLower(args[0])
		return system.ReadAndModifyHosts(func(domains map[string]string) {
			delete(domains, domain)
		})
	},
}

var hostsClearCmd = &cobra.Command{
	Use:    "clear",
	Short:  "Clear all TermWP domains from system hosts file",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return system.ReadAndModifyHosts(func(domains map[string]string) {
			for k := range domains {
				delete(domains, k)
			}
		})
	},
}

func init() {
	hostsCmd.AddCommand(hostsAddCmd)
	hostsCmd.AddCommand(hostsRemoveCmd)
	hostsCmd.AddCommand(hostsClearCmd)
	rootCmd.AddCommand(hostsCmd)
}
