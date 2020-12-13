package main

import (
	"cont"
	"cont/api"
	"cont/container"
	context "context"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"log"
	"net"
	"os"
)

type server struct {
	api.UnimplementedApiServer
}

func (s *server) Run(ctx context.Context, request *api.ContainerRequest) (*api.ContainerResponse, error) {
	id := uuid.New()
	idBytes, err := id.MarshalBinary()
	if err != nil {
		return nil, err
	}

	pipePath := cont.PipePath(id)
	if err := cont.CreatePipes(pipePath); err != nil {
		log.Printf("cannot create pipes: %v\n", err)
		return nil, err
	}

	pipes, err := cont.OpenPipes(pipePath)
	if err != nil {
		log.Printf("cannot open pipes: %v\n", err)
		return nil, err
	}

	go func() {
		defer func() {
			for _, pipe := range pipes {
				pipe.Close()
			}
			cont.RemovePipes(pipePath)
		}()
		if err = container.Run(&container.Config{
			Stdin:    pipes[0],
			Stdout:   pipes[1],
			Stderr:   pipes[2],
			Hostname: request.Hostname,
			Workdir:  request.Workdir,
			Args:     request.Args,
		}); err != nil {
			log.Printf("container run error: %v\n", err)
			return
		}
		log.Printf("container %s done\n", id.String())
	}()
	return &api.ContainerResponse{Uuid: idBytes}, nil
}

func main() {
	if os.Getpid() == 1 {
		must(container.RunChild())
		return
	}
	listen, err := net.Listen("tcp", ":9000")
	must(err)
	defer listen.Close()

	s := grpc.NewServer()
	api.RegisterApiServer(s, &server{})
	must(s.Serve(listen))
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
