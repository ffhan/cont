package main

import (
	"cont/api"
	"cont/cmd"
	"cont/container"
	"cont/daemon"
	"cont/multiplex"
	"google.golang.org/grpc"
	"net"
	"os"
)

func main() {
	if os.Getpid() == 1 {
		must(container.RunChild())
		return
	}
	listen, err := net.Listen("tcp", cmd.ApiPort)
	must(err)
	defer listen.Close()

	streamListener, err := net.Listen("tcp", cmd.StreamingPort)
	must(err)

	muxClient := multiplex.NewClient()

	daemonServer, err := daemon.NewServer(muxClient, streamListener)
	must(err)

	s := grpc.NewServer()
	api.RegisterApiServer(s, daemonServer)
	must(s.Serve(listen))
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
