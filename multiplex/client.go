// Multiplex enables multiplexing a io.ReadWriteCloser for multiple data streams.
// Each client allows reading from multiple "connections" (io.ReadWriteCloser) and writing to multiple "connections" through
// a mux.
//
// client - manages individual streams and muxes
// mux - reads from a connection and writes to appropriate streams
// stream - reads passed data and writes to a connection, each stream has an ID that connects it to other streams
//
// Multiplex allows for M:N data streams through a single data source (e.g. single TCP port), but also allows for any number of data sources.
package multiplex

import (
	"io"
	"log"
	"sync"
)

// client manages streams for use in muxes
type client struct {
	streams     map[int32]map[*stream]bool // all streams, nested maps for faster access, insertion and removal
	streamMutex sync.RWMutex               // enables concurrent stream editing
	Name        string                     // optional client name
}

// initializes a new client
func NewClient() *client {
	return &client{streams: make(map[int32]map[*stream]bool)}
}

// creates a new mux for the connection
func (c *client) NewMux(conn io.ReadWriteCloser) *mux {
	//c.logf("created a new mux\n")
	m := &mux{
		client:       c,
		conn:         conn,
		ownedStreams: make(map[*stream]bool),
	}
	go m.readIncoming()
	return m
}

func (c *client) logf(format string, args ...interface{}) {
	log.Printf("%s:"+format, append([]interface{}{c.Name}, args...)...)
}

// add a stream to a client
func (c *client) addStream(id int32, str *stream) {
	//c.logf("added stream %d", id)
	c.streamMutex.Lock()
	defer c.streamMutex.Unlock()
	if _, ok := c.streams[id]; !ok {
		c.streams[id] = make(map[*stream]bool)
	}
	c.streams[id][str] = true
}

// remove a stream from a client - the stream is not automatically closed
func (c *client) removeStream(id int32, stream *stream) {
	//c.logf("removed stream %d", id)
	c.streamMutex.Lock()
	defer c.streamMutex.Unlock()
	if _, ok := c.streams[id]; !ok {
		return
	}
	delete(c.streams[id], stream)
}

// retrieve all streams for the provided ID
func (c *client) getStreams(id int32) []*stream {
	c.streamMutex.RLock()
	defer c.streamMutex.RUnlock()
	streams, ok := c.streams[id]
	if !ok {
		return []*stream{}
	}
	result := make([]*stream, 0, len(streams))
	for s := range streams {
		result = append(result, s)
	}
	return result
}

// closes a client and all its streams
func (c *client) Close() error {
	for id, streams := range c.streams {
		for stream := range streams {
			err := stream.Close()
			if err != nil {
				c.logf("cannot close a stream %d: %v", id, err)
			}
			c.removeStream(id, stream)
		}
	}
	return nil
}
