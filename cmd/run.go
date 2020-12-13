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

		go io.Copy(os.Stdout, pipes[1]) // todo: only if not in detached mode
		go io.Copy(os.Stderr, pipes[2]) // todo: only if not in detached mode
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			line := scanner.Text() + "\n"
			_, err := pipes[0].WriteString(line)
			must(err)
		}
	},
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
}
