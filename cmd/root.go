package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
)

var (
	rootCmd = &cobra.Command{
		Use:   "cont",
		Short: "Cont is a toy container manager",
	}
)

func init() {
	rootCmd.Flags().String("host", ":9000", "defines the cont host to connect to")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
