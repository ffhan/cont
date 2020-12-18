package cmd

import (
	"cont/api"
	"context"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "run a container",
	Run: func(cmd *cobra.Command, args []string) {
		clientID := uuid.New()
		clientIDBytes, err := clientID.MarshalBinary()
		must(err)

		conn, err := GrpcDial()
		must(err)
		defer conn.Close()

		hostname, err := cmd.Flags().GetString("hostname")
		must(err)

		host, err := cmd.Flags().GetString("host")
		must(err)

		isLocal := host == Localhost

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

		signals := make(chan os.Signal, 1)
		started := make(chan bool, 1)

		go handleEvents(client, signals, started, response.Uuid)

		containerID, err := uuid.FromBytes(response.Uuid)
		must(err)

		var stdin, stdout, stderr io.ReadWriteCloser
		if isLocal {
			//fmt.Println("attaching to a local container")
			pipes := setupLocalPipes(containerID, started)

			stdin = pipes[0]
			stdout = pipes[1]
			stderr = pipes[2]
		} else {
			<-started
			//fmt.Println("attaching to a remote container")
			stdin, stdout, stderr = setupRemotePipes(client, clientID, clientIDBytes, response.Uuid)
		}
		defer closePipes(stdin, stdout, stderr)

		signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
		go func() {
			<-signals
			closePipes(stdin, stdout, stderr)
			os.Exit(0)
		}()

		var wg sync.WaitGroup

		if isDetached, err := cmd.Flags().GetBool("detached"); err == nil && !isDetached {
			attachOutput(&wg, stdout, stderr)
		} else {
			must(err)
		}
		if isInteractive, err := cmd.Flags().GetBool("it"); err == nil && isInteractive {
			setupInteractive(&wg, stdin)
		} else {
			must(err)
		}
		wg.Wait()
	},
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
