package main

import (
	"cont/api"
	"fmt"
	"github.com/google/uuid"
	"time"
)

func (s *server) sendEvent(eventChan chan *api.Event, event *api.Event) {
	select {
	case eventChan <- event:
	case <-time.After(100 * time.Millisecond):
	}
}

func (s *server) Events(request *api.EventStreamRequest, eventsServer api.Api_EventsServer) error {
	id, err := uuid.FromBytes(request.Id)
	if err != nil {
		return err
	}
	eventChan, ok := events[id] // retry finding events
	if !ok {
		return fmt.Errorf("no currently running container for ID: %v", id.String())
	}
	for event := range eventChan {
		if err := eventsServer.Send(event); err != nil {
			return err
		}
	}
	return nil
}
