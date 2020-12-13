package cmd

import (
	"bufio"
	"cont"
	"cont/api"
	"context"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"io"
	"os"
	"os/signal"
	"syscall"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "run a container",
	Run: func(cmd *cobra.Command, args []string) {
		conn, err := grpc.Dial(":9000", grpc.WithInsecure())
		must(err)
		defer conn.Close()

		client := api.NewApiClient(conn)
		response, err := client.Run(context.Background(), &api.ContainerRequest{
			Name:     "test",
			Hostname: "cont",
			Workdir:  "/home/fhancic",
			Args:     args,
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
	// todo: args, params and flags for run
	runCmd.Flags().Bool("it", false, "determines whether to connect stdin with container stdin")
	runCmd.Flags().BoolP("detached", "d", false, "run in detached mode")
}
