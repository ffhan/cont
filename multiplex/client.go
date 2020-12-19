// Multiplex enables multiplexing a io.ReadWriteCloser for multiple data streams.
// Each Client allows reading from multiple "connections" (io.ReadWriteCloser) and writing to multiple "connections" through
// a Mux.
//
// Client - manages individual streams and muxes
// Mux - reads from a connection and writes to appropriate streams
// Stream - reads passed data and writes to a connection, each Stream has an ID that connects it to other streams
//
// Multiplex allows for M:N data streams through a single data source (e.g. single TCP port), but also allows for any number of data sources.
package multiplex

import (
	"io"
	"log"
	"sync"
)

type Streamer interface {
	io.ReadCloser
	ID() string
	WriteInput([]byte) (n int, err error) // writes to stream input
}

// Client manages streams for use in muxes
type Client struct {
	muxes       map[*Mux]bool // all Muxes owned by the client
	muxMutex    sync.RWMutex
	streams     map[string]map[Streamer]bool // all streams, nested maps for faster access, insertion and removal
	streamMutex sync.RWMutex                 // enables concurrent Stream editing
	senders     map[*Sender]bool             // all senders, responsible for direct writing to all mux outputs
	senderMutex sync.RWMutex                 // enables concurrent Sender editing
	Name        string                       // optional Client name
}

// initializes a new Client
func NewClient() *Client {
	return &Client{
		streams: make(map[string]map[Streamer]bool),
		muxes:   make(map[*Mux]bool),
		senders: make(map[*Sender]bool),
	}
}

// creates a new Mux for the connection
func (c *Client) NewMux(conn io.ReadWriteCloser) *Mux {
	//c.logf("created a new Mux\n")
	m := &Mux{
		client:       c,
		conn:         conn,
		ownedStreams: make(map[Streamer]bool),
	}
	c.muxMutex.Lock()
	c.muxes[m] = true
	c.muxMutex.Unlock()
	go m.readIncoming()
	return m
}

func (c *Client) NewReceiver(id string) *Receiver {
	r := &Receiver{
		client: c,
		id:     id,
		input:  NewBlockingReader(),
	}
	c.addStream(id, r)
	return r
}

func (c *Client) NewSender(id string) *Sender {
	s := &Sender{
		client: c,
		id:     id,
	}
	c.addSender(s)
	return s
}

func (c *Client) addSender(sender *Sender) {
	c.senderMutex.Lock()
	defer c.senderMutex.Unlock()
	c.senders[sender] = true
}

func (c *Client) removeSender(sender *Sender) {
	c.senderMutex.Lock()
	defer c.senderMutex.Unlock()
	delete(c.senders, sender)
}

func (c *Client) logf(format string, args ...interface{}) {
	log.Printf("%s:"+format, append([]interface{}{c.Name}, args...)...)
}

// add a Stream to a Client
func (c *Client) addStream(id string, str Streamer) {
	//c.logf("added Stream %s", id)
	c.streamMutex.Lock()
	defer c.streamMutex.Unlock()
	if _, ok := c.streams[id]; !ok {
		c.streams[id] = make(map[Streamer]bool)
	}
	c.streams[id][str] = true
}

// remove a Stream from a Client - the Stream is not automatically closed
func (c *Client) removeStream(id string, stream Streamer) {
	//c.logf("removed Stream %s", id)
	c.streamMutex.Lock()
	defer c.streamMutex.Unlock()
	if _, ok := c.streams[id]; !ok {
		return
	}
	delete(c.streams[id], stream)
}

// retrieve all streams for the provided ID
func (c *Client) getStreams(id string) []Streamer {
	c.streamMutex.RLock()
	defer c.streamMutex.RUnlock()
	streams, ok := c.streams[id]
	if !ok {
		return []Streamer{}
	}
	result := make([]Streamer, 0, len(streams))
	for s := range streams {
		result = append(result, s)
	}
	return result
}

func (c *Client) getMuxes() []*Mux {
	c.muxMutex.RLock()
	defer c.muxMutex.RUnlock()
	muxes := make([]*Mux, 0, len(c.muxes))
	for mux := range c.muxes {
		muxes = append(muxes, mux)
	}
	return muxes
}

// closes a Client and all its streams
func (c *Client) Close() error {
	for mux := range c.muxes {
		if err := mux.Close(); err != nil {
			log.Printf("cannot close mux \"%s\": %v", mux.Name, err)
		}
	}
	for _, streams := range c.streams {
		for streamer := range streams {
			err := streamer.Close() // close all streams, including receivers (just in case)
			if err != nil {
				c.logf("cannot close a streamer: %v", err)
			}
		}
	}
	return nil
}
