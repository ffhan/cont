package cmd

import (
	"cont/api"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var attachCmd = &cobra.Command{
	Use:   "attach",
	Short: "Attach to an existing container",
	Long:  "Attach stdout and stderr to an existing container (local or remote). Interactive flag also attaches stdin.",
	Run: func(cmd *cobra.Command, args []string) {
		containerIDString := args[0]
		containerID, err := uuid.Parse(containerIDString)
		must(err)
		containerIDBytes, err := containerID.MarshalBinary()
		must(err)

		clientID := uuid.New()
		clientIDBytes, err := clientID.MarshalBinary()
		must(err)

		conn, err := GrpcDial()
		must(err)
		defer conn.Close()

		host, err := cmd.Flags().GetString("host")
		must(err)

		isInteractive, err := cmd.Flags().GetBool("it")
		must(err)

		isLocal := host == Localhost

		client := api.NewApiClient(conn)

		signals := make(chan os.Signal, 1)
		started := make(chan bool, 1)

		go handleEvents(client, signals, started, containerIDBytes)

		var stdin, stdout, stderr io.ReadWriteCloser
		if isLocal {
			//fmt.Println("attaching to a local container")
			pipes := setupLocalPipes(containerID, started)

			stdin = pipes[0]
			stdout = pipes[1]
			stderr = pipes[2]
		} else {
			//fmt.Println("attaching to a remote container")
			stdin, stdout, stderr = setupRemotePipes(client, clientID, clientIDBytes, containerIDBytes)
		}
		defer closePipes(stdin, stdout, stderr)

		var wg sync.WaitGroup

		if isInteractive {
			setupInteractive(&wg, stdin, stdout)
		} else {
			attachOutput(&wg, stdout, stderr)
			signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
			go func() {
				<-signals
				closePipes(stdin, stdout, stderr)
				os.Exit(0)
			}()
		}
		wg.Wait()
	},
}

func init() {
	rootCmd.AddCommand(attachCmd)

	attachCmd.Flags().Bool("it", false, "determines whether to connect stdin with container stdin")
}
