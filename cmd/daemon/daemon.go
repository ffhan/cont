package main

import (
	"cont"
	"cont/api"
	"cont/cmd"
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
	"time"
)

type Container struct {
	Cmd     *exec.Cmd
	Name    string
	Id      uuid.UUID
	Command string
}

type server struct {
	api.UnimplementedApiServer
}

var (
	currentlyRunning = make(map[uuid.UUID]Container)
	events           = make(map[uuid.UUID]chan *api.Event)
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
		if err := cont.RemovePipes(pipePath); err != nil {
			log.Println(err)
		}
	}()
	eventChan := make(chan *api.Event)
	events[id] = eventChan

	binaryId, err := id.MarshalBinary()
	if err != nil {
		log.Println(err)
		eventChan <- &api.Event{
			Id:      nil,
			Type:    cmd.Failed,
			Message: id.String(),
			Source:  "",
			Data:    nil,
		}
		return
	}

	eventChan <- &api.Event{
		Id:      binaryId,
		Type:    cmd.Created,
		Message: "",
		Source:  "", // todo: fill source
		Data:    nil,
	}

	containerCommand, err := container.Start(&container.Config{
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
	eventChan <- &api.Event{
		Id:      binaryId,
		Type:    cmd.Started,
		Message: "",
		Source:  "", // todo: fill source
		Data:    nil,
	}
	fmt.Printf("container %s started\n", id.String())
	mutex.Lock()

	currentlyRunning[id] = Container{
		Cmd:     containerCommand,
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
	if err = containerCommand.Wait(); err != nil {
		log.Printf("container error: %v\n", err)
		return
	}
	eventChan <- &api.Event{
		Id:      binaryId,
		Type:    cmd.Done,
		Message: "",
		Source:  "", // todo: fill source
		Data:    nil,
	}
	close(eventChan)
	delete(events, id)
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

	eventChan, ok := events[id]
	if !ok {
		return nil, errors.New("cannot find container events")
	}

	if err = c.Cmd.Process.Signal(syscall.SIGTERM); err != nil { // todo: stdin WriteCloser so that interactive clients see the kill
		return nil, err
	}

	eventChan <- &api.Event{
		Id:      killCommand.Id,
		Type:    cmd.Killed,
		Message: "",
		Source:  "",
		Data:    nil,
	}
	close(eventChan)
	delete(events, id)

	return &api.ContainerResponse{Uuid: killCommand.Id}, nil
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

func (s *server) Events(request *api.EventStreamRequest, eventsServer api.Api_EventsServer) error {
	id, err := uuid.FromBytes(request.Id)
	if err != nil {
		return err
	}
	eventChan, ok := events[id] // retry finding events
	found := false
	for i := 0; i < 100; i++ {
		if !ok {
			time.Sleep(10 * time.Millisecond)
			eventChan, ok = events[id]
			continue
		}
		found = true
		break
	}
	if !found {
		return errors.New("couldn't fetch events")
	}
	for event := range eventChan {
		if err := eventsServer.Send(event); err != nil {
			return err
		}
	}
	return nil
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
