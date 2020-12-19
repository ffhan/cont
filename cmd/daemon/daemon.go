package main

import (
	"cont"
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
	"syscall"
	"time"
)

type Container struct {
	Cmd            *exec.Cmd
	Name           string
	Id             uuid.UUID
	Command        string
	Stdin          io.ReadCloser
	Stdout, Stderr io.WriteCloser
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
		//clientID, err := uuid.FromBytes(recv.ClientId)
		//if err != nil {
		//	return err
		//}

		//s.connectionsMutex.RLock()
		//conn, ok := s.connections[clientID]
		//s.connectionsMutex.RUnlock()
		//if !ok {
		//	return fmt.Errorf("no connection for the client ID %s", clientID.String())
		//}

		containerId, err := uuid.FromBytes(recv.Id)
		if err != nil {
			return err
		}

		//mutex.Lock()
		//cont, ok := currentlyRunning[containerId]
		//if !ok {
		//	return fmt.Errorf("no currently running container %s", containerId.String())
		//}
		//mutex.Unlock()

		stdinId, stdoutId, stderrId := s.ContainerStreamIDs(containerId)

		if err = streamServer.Send(&api.StreamResponse{
			InId:  stdinId,
			OutId: stdoutId,
			ErrId: stderrId,
		}); err != nil {
			return err
		}

		fmt.Println(stdinId, stdoutId, stderrId)

		// create streams
		//r := s.muxClient.NewReceiver(stdinId)
		//go io.Copy(os.Stdout, r)
		//_ = conn.mux.NewStream(stdoutId)
		//_ = conn.mux.NewStream(stderrId)

		//fmt.Println("created new streams")
		// fixme: go run cmd/cli/cli.go run --host 127.0.0.1 --workdir /home/fhancic --hostname test2 ./skripta.sh
		// remote continuous stdout works (at least for 1 client)
		// todo: what about removing streams?
		//cont.Stdin.Add(inStream)
		//cont.Stdout.Add(outStream)
		//cont.Stderr.Add(errStream)

		//fmt.Printf("attached remote streams to container %s\n", containerId.String())
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

	pipePath := cont.PipePath(id)
	if err := cont.CreatePipes(pipePath); err != nil {
		log.Printf("cannot create pipes: %v\n", err)
		return nil, err
	}

	go s.runContainer(pipePath, request, id)
	return &api.ContainerResponse{Uuid: idBytes}, nil
}

func (s *server) runContainer(pipePath string, request *api.ContainerRequest, id uuid.UUID) {
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

	sin, sout, serr := s.ContainerStreamIDs(id)

	stdin := s.muxClient.NewReceiver(sin)
	stdout := s.muxClient.NewSender(sout)
	stderr := s.muxClient.NewSender(serr)

	defer stderr.Close()
	defer stdout.Close()
	defer stdin.Close()

	containerCommand, err := container.Start(&container.Config{
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
	eventChan <- &api.Event{
		Id:      binaryId,
		Type:    cmd.Started,
		Message: "",
		Source:  "", // todo: fill source
		Data:    nil,
	}
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
	fmt.Println("sending done event")
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
