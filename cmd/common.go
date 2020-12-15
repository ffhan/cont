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
	"google.golang.org/grpc"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"syscall"
)

func GrpcDial() (*grpc.ClientConn, error) {
	target, err := rootCmd.Flags().GetString("host")
	if err != nil {
		return nil, err
	}
	return grpc.Dial(target+ApiPort, grpc.WithInsecure())
}

const (
	Failed int32 = iota
	Created
	Started
	Done
	Killed // todo: make a distinction between done and killed
)

func handleEvents(client api.ApiClient, signals chan os.Signal, started chan bool, containerID []byte) {
	events, err := client.Events(context.Background(), &api.EventStreamRequest{Id: containerID})
	if err != nil {
		signals <- syscall.SIGTERM
		return
	}
	for {
		event, err := events.Recv()
		if err != nil {
			log.Println(err)
			signals <- syscall.SIGTERM
			return
		}
		if event.Type == Started {
			started <- true
		}
		if event.Type == Killed {
			fmt.Println("container has been killed")
		}
		if event.Type == Done || event.Type == Killed {
			signals <- syscall.SIGTERM
			return
		}
	}
}

func handleStdin(stdinPipe io.WriteCloser) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text() + "\n"
		_, err := stdinPipe.Write([]byte(line))
		must(err)
	}
}

func closePipes(stdin, stdout, stderr io.ReadWriteCloser) {
	stdin.Write([]byte("exit\n"))
	stdin.Close()
	stdout.Close()
	stderr.Close()
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func setupInteractive(cmd *cobra.Command, wg *sync.WaitGroup, stdin io.ReadWriteCloser) {
	if isInteractive, err := cmd.Flags().GetBool("it"); err == nil && isInteractive {
		wg.Add(1)
		go func() {
			handleStdin(stdin)
			wg.Done()
		}()
	} else if err != nil {
		panic(err)
	}
}

func setupDetached(cmd *cobra.Command, wg *sync.WaitGroup, stdout, stderr io.ReadWriteCloser) {
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
}

func setupLocalPipes(containerID uuid.UUID, started chan bool) [3]*os.File {
	<-started
	pipes, err := cont.OpenPipes(cont.PipePath(containerID))
	must(err)

	return pipes
}

func setupRemotePipes(client api.ApiClient, clientID uuid.UUID, clientIDBytes, containerIDBytes []byte, started chan bool) (io.ReadWriteCloser, io.ReadWriteCloser, io.ReadWriteCloser) {
	<-started
	streamingConn, err := net.Dial("tcp", StreamingPort)
	must(err)

	// send our client ID
	must(gob.NewEncoder(streamingConn).Encode(clientID))

	muxClient := multiplex.NewClient()
	mux := muxClient.NewMux(streamingConn)

	streamRequestClient, err := client.RequestStream(context.Background())
	must(err)
	must(streamRequestClient.Send(&api.StreamRequest{
		Id:       containerIDBytes,
		ClientId: clientIDBytes,
	}))
	streamResponse, err := streamRequestClient.Recv()
	must(err)

	fmt.Println("response: ", streamResponse)
	return mux.NewStream(streamResponse.InId), mux.NewStream(streamResponse.OutId), mux.NewStream(streamResponse.ErrId)
}
