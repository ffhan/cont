package main

import (
	"cont/api"
	"errors"
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
