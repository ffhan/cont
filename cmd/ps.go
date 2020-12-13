package cmd

import (
	"cont/api"
	"context"
	"fmt"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"os"
)

var psCmd = &cobra.Command{
	Use:   "ps",
	Short: "list active processes",
	Run: func(cmd *cobra.Command, args []string) {
		dial, err := GrpcDial()
		must(err)

		client := api.NewApiClient(dial)
		processes, err := client.Ps(context.Background(), &api.Empty{})
		must(err)

		must(printProcesses(processes))
	},
}

func printProcesses(processes *api.ActiveProcesses) error {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"UUID", "CMD", "PID"})
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	for _, proc := range processes.Processes {
		table.Append([]string{proc.Id, proc.Cmd, fmt.Sprint(proc.Pid)})
	}
	table.Render()
	return nil
}

func init() {
	rootCmd.AddCommand(psCmd)
}
