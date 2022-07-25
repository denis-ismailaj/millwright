package cmd

import (
	"github.com/spf13/cobra"
)

var (
	// RootCmd is the root command for the internal.
	RootCmd = &cobra.Command{
		Use:   "internal",
		Short: "An internal that manages the components of the test task.",
		Run:   root,
	}
)

func root(cmd *cobra.Command, _ []string) {
	_ = cmd.Help()
}
