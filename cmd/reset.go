package cmd

import (
	"github.com/spf13/cobra"
)

var resetCmd = &cobra.Command{
	Use:     "reset",
	Aliases: []string{"factory-reset"},
	Short:   "Factory reset: clear inbounds, clients, staff, and restore default settings & admin user",
	Run: func(cmd *cobra.Command, args []string) {
		FactoryReset()
	},
}

func init() {
	rootCmd.AddCommand(resetCmd)
}
