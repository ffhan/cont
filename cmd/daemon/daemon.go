package main

import (
	"cont"
	"cont/api"
	"cont/container"
	context "context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
)

type server struct {
	api.UnimplementedApiServer
}

type Container struct {
	Cmd     *exec.Cmd
	Name    string
	Id      uuid.UUID
	Command string
}

var (
	currentlyRunning = make(map[uuid.UUID]Container)
	mutex            sync.Mutex
)

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

	go s.runContainer(pipes, pipePath, request, id)
	return &api.ContainerResponse{Uuid: idBytes}, nil
}

func (s *server) runContainer(pipes [3]*os.File, pipePath string, request *api.ContainerRequest, id uuid.UUID) {
	defer func() {
		for _, pipe := range pipes {
			pipe.Close()
		}
		cont.RemovePipes(pipePath)
	}()
	cmd, err := container.Start(&container.Config{
		Stdin:    pipes[0],
		Stdout:   pipes[1],
		Stderr:   pipes[2],
		Hostname: request.Hostname,
		Workdir:  request.Workdir,
		Cmd:      request.Cmd,
		Args:     request.Args,
	})
	if err != nil {
		log.Printf("container start error: %v\n", err)
		return
	}
	fmt.Printf("container %s started\n", id.String())
	mutex.Lock()

	currentlyRunning[id] = Container{
		Cmd:     cmd,
		Name:    request.Name,
		Id:      id,
		Command: strings.Join(append([]string{request.Cmd}, request.Args...), " "),
	}
	mutex.Unlock()
	defer func() {
		mutex.Lock()
		defer mutex.Unlock()
		delete(currentlyRunning, id)
	}()
	if err = cmd.Wait(); err != nil {
		log.Printf("container error: %v\n", err)
		return
	}
	log.Printf("container %s done\n", id.String())
}

func (s *server) Kill(ctx context.Context, killCommand *api.KillCommand) (*api.ContainerResponse, error) {
	id, err := uuid.ParseBytes(killCommand.Id)
	if err != nil {
		return nil, err
	}
	c, ok := currentlyRunning[id]
	if !ok {
		return nil, errors.New("container doesn't exist")
	}
	if err = c.Cmd.Process.Signal(syscall.SIGTERM); err != nil { // todo: stdin WriteCloser so that interactive clients see the kill
		return nil, err
	}
	idBytes, err := id.MarshalBinary()
	if err != nil {
		return nil, err
	}
	return &api.ContainerResponse{Uuid: idBytes}, nil
}

func (s *server) Ps(ctx context.Context, empty *api.Empty) (*api.ActiveProcesses, error) {
	processes := make([]*api.Process, 0, len(currentlyRunning))
	for id, c := range currentlyRunning {
		processes = append(processes, &api.Process{
			Id:   id.String(),
			Name: c.Name,
			Cmd:  c.Command,
			Pid:  int64(c.Cmd.Process.Pid),
		})
	}
	result := &api.ActiveProcesses{Processes: processes}
	return result, nil
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
