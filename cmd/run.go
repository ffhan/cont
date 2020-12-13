package cmd

import (
	"bufio"
	"cont"
	"cont/api"
	"context"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"io"
	"os"
	"os/signal"
	"syscall"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "run a container",
	Run: func(cmd *cobra.Command, args []string) {
		conn, err := GrpcDial()
		must(err)
		defer conn.Close()

		hostname, err := cmd.Flags().GetString("hostname")
		must(err)

		workdir, err := cmd.Flags().GetString("workdir")
		must(err)

		name, err := cmd.Flags().GetString("name")
		must(err)

		client := api.NewApiClient(conn)
		response, err := client.Run(context.Background(), &api.ContainerRequest{
			Name:     name,
			Hostname: hostname,
			Workdir:  workdir,
			Cmd:      args[0],
			Args:     args[1:],
		})
		must(err)

		id, err := uuid.FromBytes(response.Uuid)
		must(err)
		pipes, err := cont.OpenPipes(cont.PipePath(id))
		must(err)

		defer closePipes(pipes)

		signals := make(chan os.Signal, 1)
		signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)

		go func() {
			<-signals
			closePipes(pipes)
			os.Exit(0)
		}()

		if isDetached, err := cmd.Flags().GetBool("detached"); err == nil && !isDetached {
			go io.Copy(os.Stdout, pipes[1])
			go io.Copy(os.Stderr, pipes[2])
		} else if err != nil {
			panic(err)
		}
		if isInteractive, err := cmd.Flags().GetBool("it"); err == nil && isInteractive {
			handleStdin(pipes[0])
		} else if err != nil {
			panic(err)
		}
	},
}

func handleStdin(stdinPipe *os.File) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text() + "\n"
		_, err := stdinPipe.WriteString(line)
		must(err)
	}
}

func closePipes(pipes [3]*os.File) {
	pipes[0].WriteString("exit\n")
	for _, pipe := range pipes {
		pipe.Close()
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func init() {
	rootCmd.AddCommand(runCmd)

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "cont"
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "/"
	}

	runCmd.Flags().Bool("it", false, "determines whether to connect stdin with container stdin")
	runCmd.Flags().BoolP("detached", "d", false, "run in detached mode")
	runCmd.Flags().String("hostname", hostname, "sets container hostname")
	runCmd.Flags().String("workdir", homeDir, "sets container workdir")
	runCmd.Flags().String("name", "", "sets container name")
}
