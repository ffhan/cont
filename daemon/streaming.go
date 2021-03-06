package daemon

import (
	"cont/api"
	"cont/multiplex"
	"encoding/gob"
	"fmt"
	"github.com/google/uuid"
	"log"
	"net"
)

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
		mux.AddOnClose(func(mux *multiplex.Mux) {
			log.Printf("removing mux connection \"%s\"", mux.Name)
			s.connectionsMutex.Lock()
			defer s.connectionsMutex.Unlock()

			conn, ok := s.connections[clientID]
			if !ok {
				log.Printf("no connection for client ID %s found", clientID.String())
				return
			}

			if err := s.updateContainer(conn.ContainerID, func(c *Container) error {
				delete(c.Streamers, clientID)
				return nil
			}); err != nil {
				log.Printf("cannot remove streamer: %v", err)
			}

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
		log.Println("received a stream request")

		clientID, err := uuid.FromBytes(recv.ClientId)
		if err != nil {
			return err
		}

		containerId, err := uuid.FromBytes(recv.Id)
		if err != nil {
			return err
		}

		if err = s.updateContainer(containerId, func(c *Container) error {
			s.connectionsMutex.RLock()
			defer s.connectionsMutex.RUnlock()

			stream, ok := s.connections[clientID]
			if !ok {
				return fmt.Errorf("no client with ID %s currently streaming", clientID.String())
			}
			stream.ContainerID = containerId

			c.Streamers[clientID] = stream
			return nil
		}); err != nil {
			return fmt.Errorf("cannot update container: %v", err)
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
