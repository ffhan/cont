package cmd

import (
	"cont"
	"cont/api"
	"cont/multiplex"
	"context"
	"encoding/gob"
	"fmt"
	"github.com/google/uuid"
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
	go io.Copy(stdinPipe, os.Stdin)
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

func setupInteractive(wg *sync.WaitGroup, stdin io.ReadWriteCloser) {
	wg.Add(1)
	go func() {
		handleStdin(stdin)
		wg.Done()
	}()
}

func attachOutput(wg *sync.WaitGroup, stdout, stderr io.ReadWriteCloser) {
	wg.Add(2)
	go func() {
		io.Copy(os.Stdout, stdout)
		wg.Done()
	}()
	go func() {
		io.Copy(os.Stderr, stderr)
		wg.Done()
	}()
}

func setupLocalPipes(containerID uuid.UUID, started chan bool) [3]*os.File {
	<-started
	pipes, err := cont.OpenPipes(cont.PipePath(containerID))
	must(err)

	return pipes
}

func setupRemotePipes(client api.ApiClient, clientID uuid.UUID, clientIDBytes, containerIDBytes []byte) (io.ReadWriteCloser, io.ReadWriteCloser, io.ReadWriteCloser) {
	//fmt.Println("initiating remote pipe setup")
	streamingConn, err := net.Dial("tcp", StreamingPort)
	must(err)
	//fmt.Println("tcp stream dial success")

	// send our client ID
	must(gob.NewEncoder(streamingConn).Encode(clientID))

	//fmt.Println("sent client ID")

	muxClient := multiplex.NewClient()
	mux := muxClient.NewMux(streamingConn)

	//fmt.Println("mux set up")

	streamRequestClient, err := client.RequestStream(context.Background())
	//fmt.Println("setup request stream")
	must(err)
	must(streamRequestClient.Send(&api.StreamRequest{
		Id:       containerIDBytes,
		ClientId: clientIDBytes,
	}))
	//fmt.Println("sent a stream request")
	streamResponse, err := streamRequestClient.Recv()
	must(err)

	//fmt.Println("response: ", streamResponse)
	return mux.NewStream(streamResponse.InId), mux.NewStream(streamResponse.OutId), mux.NewStream(streamResponse.ErrId)
}
