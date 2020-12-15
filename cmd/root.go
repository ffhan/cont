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

const (
	ApiPort       = ":9000"
	StreamingPort = ":9001"
	Localhost     = "localhost"
)

func init() {
	rootCmd.PersistentFlags().String("host", Localhost, "defines the cont host to connect to")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
