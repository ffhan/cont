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
