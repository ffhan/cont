package daemon

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
	muxClient             *multiplex.Client
	connections           map[uuid.UUID]*streamConn
	currentlyRunning      map[uuid.UUID]*Container
	events                map[uuid.UUID]chan *api.Event
	connectionsMutex      sync.RWMutex
	currentlyRunningMutex sync.RWMutex
	eventMutex            sync.RWMutex
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
	s := &server{
		muxClient:        muxClient,
		connections:      make(map[uuid.UUID]*streamConn),
		currentlyRunning: make(map[uuid.UUID]*Container),
		events:           make(map[uuid.UUID]chan *api.Event),
	}
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
