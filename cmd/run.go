package cmd

import (
	"cont/api"
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
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

		isInteractive, err := cmd.Flags().GetBool("it")
		must(err)

		isDetached, err := cmd.Flags().GetBool("detached")
		must(err)

		isLocal := host == Localhost

		workdir, err := cmd.Flags().GetString("workdir")
		must(err)

		name, err := cmd.Flags().GetString("name")
		must(err)

		shareNSID, err := cmd.Flags().GetString("share-ns")
		must(err)

		var shareNS int
		var shareID []byte
		if shareNSID != "" {
			shareUUID, err := uuid.Parse(shareNSID)
			if err != nil {
				panic(fmt.Errorf("cannot parse share ID: %w", err))
			}
			shareID, err = shareUUID.MarshalBinary()
			if err != nil {
				panic(fmt.Errorf("cannot marshal share UUID to binary: %w", err))
			}
			shareNS = syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET | syscall.CLONE_NEWUSER | syscall.CLONE_NEWIPC | unix.CLONE_NEWCGROUP
		}

		client := api.NewApiClient(conn)
		cReq := &api.ContainerRequest{
			Name:     name,
			Hostname: hostname,
			Workdir:  workdir,
			Cmd:      args[0],
			Args:     args[1:],
			Opts: &api.ContainerOpts{
				Interactive: isInteractive,
				ShareOpts: &api.ShareNSOpts{
					Flags:   int64(shareNS),
					ShareID: shareID,
				},
			},
		}
		//log.Printf("container request: %+v", cReq)
		response, err := client.Run(context.Background(), cReq)
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

		var wg sync.WaitGroup

		if isDetached {
			return
		}

		if isInteractive {
			// if interactive mode
			setupInteractive(&wg, stdin, stdout)
		} else {
			// if no interactive mode, just output
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
	runCmd.Flags().String("share-ns", "", "selects which container to share namespaces with. The containers have to be co-located (on the same host). Will not try to share mount NS.")
	runCmd.Flags().String("hostname", hostname, "sets container hostname")
	runCmd.Flags().String("workdir", homeDir, "sets container workdir")
	runCmd.Flags().String("name", "", "sets container name")
}
