package main

import (
	"cont/api"
	"cont/cmd"
	"cont/container"
	"cont/multiplex"
	"context"
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
	eventChan := s.createEventChan(id)

	binaryId, err := id.MarshalBinary()
	if err != nil {
		log.Printf("cannot marshal UUID to binary: %v", err)
		s.sendFailedEvent(eventChan, id, err)
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
	stdin, stdout, stderr := s.setupStd(sin, sout, serr)

	defer s.closeStd(stdin, id, stdout, stderr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	defer s.closeEventChan(eventChan, id)

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
		s.sendFailedEvent(eventChan, id, err)
		return
	}
	s.sendEvent(eventChan, &api.Event{
		Id:      binaryId,
		Type:    cmd.Started,
		Message: "",
		Source:  "", // todo: fill source
		Data:    nil,
	})
	log.Printf("container %s started\n", id.String())

	newContainer := &Container{
		Cmd:     containerCommand,
		Name:    request.Name,
		Id:      id,
		Command: strings.Join(append([]string{request.Cmd}, request.Args...), " "),
		Stdin:   stdin,
		Stdout:  stdout,
		Stderr:  stderr,
		cancel:  cancel,
	}
	s.addContainer(newContainer)
	defer s.removeContainer(id)

	if err = containerCommand.Wait(); err != nil {
		log.Printf("wait error (container is dead): %v\n", err)
		log.Printf("container %s killed \n", id.String())
		return
	}
	log.Println("sending done event")
	s.sendEvent(eventChan, &api.Event{
		Id:      binaryId,
		Type:    cmd.Done,
		Message: "",
		Source:  "", // todo: fill source
		Data:    nil,
	})
	log.Printf("container %s done\n", id.String())
}

func (s *server) createEventChan(id uuid.UUID) chan *api.Event {
	eventChan := make(chan *api.Event)
	events[id] = eventChan
	return eventChan
}

func (s *server) closeEventChan(eventChan chan *api.Event, id uuid.UUID) {
	close(eventChan)
	delete(events, id)
}

func (s *server) sendFailedEvent(eventChan chan *api.Event, id uuid.UUID, err error) {
	s.sendEvent(eventChan, &api.Event{
		Id:      nil,
		Type:    cmd.Failed,
		Message: id.String(),
		Source:  "",
		Data:    []byte(err.Error()),
	})
}

func (s *server) removeContainer(id uuid.UUID) {
	mutex.Lock()
	defer mutex.Unlock()
	delete(currentlyRunning, id)
}

func (s *server) addContainer(newContainer *Container) {
	mutex.Lock()
	currentlyRunning[newContainer.Id] = newContainer
	mutex.Unlock()
}

func (s *server) setupStd(sin string, sout string, serr string) (*multiplex.Receiver, *multiplex.Sender, *multiplex.Sender) {
	stdin := s.muxClient.NewReceiver(sin)
	stdout := s.muxClient.NewSender(sout)
	stderr := s.muxClient.NewSender(serr)
	return stdin, stdout, stderr
}

func (s *server) closeStd(stdin *multiplex.Receiver, id uuid.UUID, stdout *multiplex.Sender, stderr *multiplex.Sender) {
	if err := stdin.Close(); err != nil {
		log.Printf("cannot close stdin for container %s: %v", id.String(), err)
	}
	if err := stdout.Close(); err != nil {
		log.Printf("cannot close stdout for container %s: %v", id.String(), err)
	}
	if err := stderr.Close(); err != nil {
		log.Printf("cannot close stderr for container %s: %v", id.String(), err)
	}
}
