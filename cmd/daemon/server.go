package main

import (
	"cont/api"
	"cont/multiplex"
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"io"
	"net"
	"os/exec"
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

var (
	currentlyRunning = make(map[uuid.UUID]*Container) // todo: transfer to a server struct
	events           = make(map[uuid.UUID]chan *api.Event)
	mutex            sync.Mutex
)

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
