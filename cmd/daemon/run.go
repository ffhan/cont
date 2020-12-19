package main

import (
	"cont/api"
	"cont/cmd"
	"cont/container"
	"context"
	"fmt"
	"github.com/google/uuid"
	"log"
	"strings"
)

func (s *server) Run(ctx context.Context, request *api.ContainerRequest) (*api.ContainerResponse, error) {
	id := uuid.New()
	idBytes, err := id.MarshalBinary()
	if err != nil {
		return nil, err
	}

	go s.runContainer(request, id)
	return &api.ContainerResponse{Uuid: idBytes}, nil
}

func (s *server) runContainer(request *api.ContainerRequest, id uuid.UUID) {
	eventChan := make(chan *api.Event)
	events[id] = eventChan

	binaryId, err := id.MarshalBinary()
	if err != nil {
		log.Println(err)
		s.sendEvent(eventChan, &api.Event{
			Id:      nil,
			Type:    cmd.Failed,
			Message: id.String(),
			Source:  "",
			Data:    nil,
		})
		return
	}

	s.sendEvent(eventChan, &api.Event{
		Id:      binaryId,
		Type:    cmd.Created,
		Message: "",
		Source:  "", // todo: fill source
		Data:    nil,
	})

	sin, sout, serr := s.ContainerStreamIDs(id)

	stdin := s.muxClient.NewReceiver(sin)
	stdout := s.muxClient.NewSender(sout)
	stderr := s.muxClient.NewSender(serr)

	defer stderr.Close()
	defer stdout.Close()
	defer stdin.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	defer func() {
		close(eventChan)
		delete(events, id)
	}()

	containerCommand, err := container.Start(ctx, &container.Config{
		Stdin:    stdin,
		Stdout:   stdout,
		Stderr:   stderr,
		Hostname: request.Hostname,
		Workdir:  request.Workdir,
		Cmd:      request.Cmd,
		Args:     request.Args,
	})
	if err != nil {
		log.Printf("container start error: %v\n", err)
		return
	}
	s.sendEvent(eventChan, &api.Event{
		Id:      binaryId,
		Type:    cmd.Started,
		Message: "",
		Source:  "", // todo: fill source
		Data:    nil,
	})
	fmt.Printf("container %s started\n", id.String())

	mutex.Lock()
	currentlyRunning[id] = &Container{
		Cmd:     containerCommand,
		Name:    request.Name,
		Id:      id,
		Command: strings.Join(append([]string{request.Cmd}, request.Args...), " "),
		Stdin:   stdin,
		Stdout:  stdout,
		Stderr:  stderr,
		cancel:  cancel,
	}
	mutex.Unlock()

	defer func() {
		mutex.Lock()
		defer mutex.Unlock()
		delete(currentlyRunning, id)
	}()
	if err = containerCommand.Wait(); err != nil { // fixme: this hangs if process has been killed directly!
		log.Printf("wait error (container is dead): %v\n", err)
		log.Printf("container %s killed \n", id.String())
		return
	}
	fmt.Println("sending done event")
	s.sendEvent(eventChan, &api.Event{
		Id:      binaryId,
		Type:    cmd.Done,
		Message: "",
		Source:  "", // todo: fill source
		Data:    nil,
	})
	log.Printf("container %s done\n", id.String())
}
