package cmd

import (
	"bufio"
	"cont"
	"cont/api"
	"cont/multiplex"
	"context"
	"encoding/gob"
	"fmt"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"io"
	"log"
	"net"
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
		signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)

		var stdin, stdout, stderr io.ReadWriteCloser
		if isLocal {
			fmt.Println("attaching to a local container")
			id, err := uuid.FromBytes(response.Uuid)
			must(err)
			pipes, err := cont.OpenPipes(cont.PipePath(id))
			must(err)

			defer closePipes(pipes)

			go func() {
				<-signals
				closePipes(pipes)
				os.Exit(0)
			}()

			stdin = pipes[0]
			stdout = pipes[1]
			stderr = pipes[2]
		} else {
			fmt.Println("attaching to a remote container")
			streamingConn, err := net.Dial("tcp", StreamingPort)
			must(err)

			// send our client ID
			must(gob.NewEncoder(streamingConn).Encode(clientID))

			muxClient := multiplex.NewClient()
			mux := muxClient.NewMux(streamingConn)

			streamRequestClient, err := client.RequestStream(context.Background())
			must(err)
			must(streamRequestClient.Send(&api.StreamRequest{
				Id:       response.Uuid,
				ClientId: clientIDBytes,
			}))
			streamResponse, err := streamRequestClient.Recv()
			must(err)

			stdin = mux.NewStream(streamResponse.InId)
			stdout = mux.NewStream(streamResponse.OutId)
			stderr = mux.NewStream(streamResponse.ErrId)
		}

		go func() {
			events, err := client.Events(context.Background(), &api.EventStreamRequest{Id: response.Uuid})
			if err != nil {
				signals <- syscall.SIGTERM
				return
			}
			for {
				event, err := events.Recv()
				if err != nil {
					log.Println(err)
					signals <- syscall.SIGTERM
					break
				}
				if event.Type == Killed {
					fmt.Println("container has been killed")
				}
				if event.Type == Done || event.Type == Killed {
					signals <- syscall.SIGTERM
					return
				}
			}
		}()

		var wg sync.WaitGroup

		if isDetached, err := cmd.Flags().GetBool("detached"); err == nil && !isDetached {
			wg.Add(2)
			go func() {
				io.Copy(os.Stdout, stdout)
				wg.Done()
			}()
			go func() {
				io.Copy(os.Stderr, stderr)
				wg.Done()
			}()
		} else if err != nil {
			panic(err)
		}
		if isInteractive, err := cmd.Flags().GetBool("it"); err == nil && isInteractive {
			wg.Add(1)
			go func() {
				handleStdin(stdin)
				wg.Done()
			}()
		} else if err != nil {
			panic(err)
		}
		wg.Wait()
	},
}

func handleStdin(stdinPipe io.WriteCloser) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text() + "\n"
		_, err := stdinPipe.Write([]byte(line))
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
