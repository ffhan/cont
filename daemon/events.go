package daemon

import (
	"cont/api"
	"cont/cmd"
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
	eventChan, ok := s.getEventChan(id)
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

func (s *server) getEventChan(id uuid.UUID) (chan *api.Event, bool) {
	s.eventMutex.RLock()
	defer s.eventMutex.RUnlock()

	eventChan, ok := s.events[id]
	return eventChan, ok
}

func (s *server) createEventChan(id uuid.UUID) chan *api.Event {
	s.eventMutex.Lock()
	defer s.eventMutex.Unlock()

	eventChan := make(chan *api.Event)
	s.events[id] = eventChan
	return eventChan
}

func (s *server) closeEventChan(eventChan chan *api.Event, id uuid.UUID) {
	s.eventMutex.Lock()
	defer s.eventMutex.Unlock()

	close(eventChan)
	delete(s.events, id)
}

func (s *server) sendFailedEvent(eventChan chan *api.Event, id uuid.UUID, err error) {
	s.sendEvent(eventChan, &api.Event{
		Id:      nil,
		Type:    cmd.Failed,
		Message: id.String(),
		Source:  "",
		Data:    []byte(err.Error()),
	})
}
