package main

import (
	"cont/api"
	"cont/cmd"
	"context"
	"errors"
	"github.com/google/uuid"
)

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

	s.killContainer(c, eventChan, killCommand.Id)

	return &api.ContainerResponse{Uuid: killCommand.Id}, nil
}

func (s *server) killContainer(c *Container, eventChan chan *api.Event, containerID []byte) {
	// close all container streams
	_ = c.Stdin.Close()
	_ = c.Stdout.Close()
	_ = c.Stderr.Close()
	// kill the container with context cancellation
	c.cancel()

	// send the event that the container has been killed
	s.sendEvent(eventChan, &api.Event{ // todo: stream the event to all attached clients
		Id:      containerID,
		Type:    cmd.Killed,
		Message: "",
		Source:  "",
		Data:    nil,
	})
}
