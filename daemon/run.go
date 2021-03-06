package daemon

import (
	"cont/api"
	"cont/cmd"
	"cont/container"
	"cont/multiplex"
	"context"
	"fmt"
	"github.com/google/uuid"
	"log"
	"path/filepath"
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
	defer s.closeEventChan(eventChan, id)

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

	shareConfig, err := s.setupShareConfig(request)
	if err != nil {
		log.Printf("cannot setup share config: %v", err)
		s.sendFailedEvent(eventChan, id, err)
		return
	}

	containerCommand, err := container.Start(ctx, &container.Config{
		Stdin:                 stdin,
		Stdout:                stdout,
		Stderr:                stderr,
		Hostname:              request.Hostname,
		Workdir:               request.Workdir,
		Cmd:                   request.Cmd,
		Args:                  request.Args,
		Interactive:           request.Opts.Interactive,
		SharedNamespaceConfig: shareConfig,
		Logging: container.LoggingConfig{
			Path: filepath.Join("./logs", id.String()), // todo: use /var/log/cont/<container_id> for logs
		},
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
		Cmd:       containerCommand,
		Name:      request.Name,
		Id:        id,
		Command:   strings.Join(append([]string{request.Cmd}, request.Args...), " "),
		Stdin:     stdin,
		Stdout:    stdout,
		Stderr:    stderr,
		cancel:    cancel,
		Streamers: make(map[uuid.UUID]*streamConn),
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

func (s *server) setupShareConfig(request *api.ContainerRequest) (container.SharedNamespaceConfig, error) {
	var result container.SharedNamespaceConfig
	doShare := request.Opts.ShareOpts.Flags != 0
	if !doShare {
		return result, nil
	}
	result.Flags = int(request.Opts.ShareOpts.Flags) // might have problems with truncating

	containerID, err := uuid.FromBytes(request.Opts.ShareOpts.ShareID)
	if err != nil {
		return result, err
	}

	c, ok := s.getContainer(containerID)
	if !ok {
		return result, fmt.Errorf("container %s is not currently running", containerID.String())
	}

	result.PID = c.Cmd.Process.Pid
	return result, nil
}

func (s *server) removeContainer(id uuid.UUID) {
	s.currentlyRunningMutex.Lock()
	defer s.currentlyRunningMutex.Unlock()

	c, ok := s.currentlyRunning[id]
	if !ok {
		return
	}
	for streamId, streamer := range c.Streamers {
		if err := streamer.Close(); err != nil {
			log.Printf("cannot close streamer %s: %v", streamId.String(), err)
		}
	}

	delete(s.currentlyRunning, id)
}

func (s *server) addContainer(newContainer *Container) {
	s.currentlyRunningMutex.Lock()
	defer s.currentlyRunningMutex.Unlock()

	s.currentlyRunning[newContainer.Id] = newContainer
}

func (s *server) getContainer(id uuid.UUID) (*Container, bool) {
	s.currentlyRunningMutex.RLock()
	defer s.currentlyRunningMutex.RUnlock()

	c, ok := s.currentlyRunning[id]
	return c, ok
}

func (s *server) updateContainer(id uuid.UUID, updateFunc func(c *Container) error) error {
	c, ok := s.getContainer(id)
	if !ok {
		return fmt.Errorf("container %s doesn't exist", id.String())
	}
	s.currentlyRunningMutex.Lock()
	defer s.currentlyRunningMutex.Unlock()
	return updateFunc(c)
}

func (s *server) getCurrentlyRunning() []*Container {
	s.currentlyRunningMutex.RLock()
	s.currentlyRunningMutex.RUnlock()

	containers := make([]*Container, 0, len(s.currentlyRunning))
	for _, c := range s.currentlyRunning {
		containers = append(containers, c)
	}
	return containers
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
