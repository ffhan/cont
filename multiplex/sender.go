package multiplex

import (
	"io"
	"log"
	"sync"
)

// sender sends data only to other streams, it doesn't have it's own output connection.
type Sender struct {
	client *Client
	id     string // Sender stream ID
	closed bool
}

func (s *Sender) Write(p []byte) (n int, err error) {
	if s.closed {
		return 0, io.EOF
	}
	var wg sync.WaitGroup
	muxes := s.client.getMuxes()
	wg.Add(len(muxes))
	for _, mux := range muxes {
		mux := mux
		go func() {
			defer wg.Done()
			if _, err := mux.write(s.id, p); err != nil { // todo: remove mux (& streams) on failed writes
				log.Printf("cannot write to mux \"%s\": %v", mux.Name, err)
				if err := mux.Close(); err != nil {
					log.Printf("cannot close Mux %s: %v", mux.Name, err)
				}
			}
		}()
	}
	wg.Wait()
	return len(p), nil
}

func (s *Sender) Close() error {
	s.closed = true
	s.client.removeSender(s)
	return nil
}
