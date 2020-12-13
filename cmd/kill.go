package cmd

import (
	"cont/api"
	"context"
	"fmt"
	"github.com/spf13/cobra"
)

var killCmd = &cobra.Command{
	Use:   "kill",
	Short: "kill a container by its ID",
	Run: func(cmd *cobra.Command, args []string) {
		conn, err := GrpcDial()
		must(err)

		client := api.NewApiClient(conn)
		response, err := client.Kill(context.Background(), &api.KillCommand{Id: []byte(args[0])})
		must(err)

		fmt.Print(response)
	},
}

func init() {
	rootCmd.AddCommand(killCmd)
}
