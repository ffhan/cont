package main

import (
	"cont/api"
	"cont/cmd"
	"cont/container"
	"cont/multiplex"
	context "context"
	"encoding/gob"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type Container struct {
	Cmd            *exec.Cmd
	Name           string
	Id             uuid.UUID
	Command        string
	Stdin          io.ReadCloser
	Stdout, Stderr io.WriteCloser
	cancel         context.CancelFunc
}

type server struct {
	api.UnimplementedApiServer
	muxClient        *multiplex.Client
	connections      map[uuid.UUID]*streamConn
	connectionsMutex sync.RWMutex
}

type streamConn struct {
	net.Conn
	mux *multiplex.Mux
}

func NewServer(muxClient *multiplex.Client, connectionListener net.Listener) (*server, error) {
	if muxClient == nil {
		return nil, errors.New("muxClient is nil")
	}
	if connectionListener == nil {
		return nil, errors.New("connectionListener is nil")
	}
	s := &server{muxClient: muxClient, connections: make(map[uuid.UUID]*streamConn)}
	go s.acceptStreamConnections(connectionListener)

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			fmt.Println(s.connections)
		}
	}()

	return s, nil
}

func (s *server) acceptStreamConnections(listener net.Listener) {
	for {
		accept, err := listener.Accept()
		if err != nil {
			log.Printf("cannot accept a connection: %v", err)
			return
		}
		log.Println("accepted a streaming connection")

		clientID := uuid.UUID{}
		if err = gob.NewDecoder(accept).Decode(&clientID); err != nil {
			log.Printf("cannot decode a client ID: %v", err)
			continue
		}
		s.connectionsMutex.Lock()
		mux := s.muxClient.NewMux(accept)
		mux.Name = clientID.String()
		s.connections[clientID] = &streamConn{
			Conn: accept,
			mux:  mux,
		}
		mux.AddOnClose(func(mux *multiplex.Mux) { // todo: implement ping stream for each mux to constantly check connections and remove unused ones
			log.Println("removing mux connection")
			s.connectionsMutex.Lock()
			defer s.connectionsMutex.Unlock()
			delete(s.connections, clientID)
		})

		s.connectionsMutex.Unlock()
		log.Printf("added mux to connections for client %s\n", clientID.String())
	}
}

func (s *server) RequestStream(streamServer api.Api_RequestStreamServer) error {
	for {
		recv, err := streamServer.Recv()
		if err != nil {
			return err
		}
		fmt.Println("received a stream request")

		containerId, err := uuid.FromBytes(recv.Id)
		if err != nil {
			return err
		}

		stdinId, stdoutId, stderrId := s.ContainerStreamIDs(containerId)

		if err = streamServer.Send(&api.StreamResponse{
			InId:  stdinId,
			OutId: stdoutId,
			ErrId: stderrId,
		}); err != nil {
			return err
		}
	}
}

func (s *server) ContainerStreamIDs(containerId uuid.UUID) (string, string, string) {
	cIDString := containerId.String()
	stdinId := cIDString + "-0"
	stdoutId := cIDString + "-1"
	stderrId := cIDString + "-2"
	return stdinId, stdoutId, stderrId
}

var (
	currentlyRunning = make(map[uuid.UUID]*Container) // todo: transfer to a server struct
	events           = make(map[uuid.UUID]chan *api.Event)
	mutex            sync.Mutex
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

func (s *server) sendEvent(eventChan chan *api.Event, event *api.Event) {
	select {
	case eventChan <- event:
	case <-time.After(100 * time.Millisecond):
	}
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

	_ = c.Stdin.Close()
	_ = c.Stdout.Close()
	_ = c.Stderr.Close()
	c.cancel()

	s.sendEvent(eventChan, &api.Event{
		Id:      killCommand.Id,
		Type:    cmd.Killed,
		Message: "",
		Source:  "",
		Data:    nil,
	})

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
	listen, err := net.Listen("tcp", cmd.ApiPort)
	must(err)
	defer listen.Close()

	streamListener, err := net.Listen("tcp", cmd.StreamingPort)
	must(err)

	muxClient := multiplex.NewClient()

	daemonServer, err := NewServer(muxClient, streamListener)
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
