package multiplex

import (
	"cont/api"
	"errors"
	"github.com/golang/protobuf/proto"
	"io"
	"log"
	"sync"
)

// mux multiplexes a connection to a number of streams.
// Every mux handles only one connection.
type mux struct {
	ownedStreams map[*stream]bool
	streamMutex  sync.RWMutex
	client       *client
	Name         string
	conn         io.ReadWriteCloser
}

func (m *mux) logf(format string, args ...interface{}) {
	m.client.logf("%s: "+format, append([]interface{}{m.Name}, args...)...)
}

// read incoming data from a connection and pass it to appropriate streams
func (m *mux) readIncoming() {
	buffer := make([]byte, 2048) // todo: is 2k memory enough?
	for {
		read, err := m.conn.Read(buffer)
		data := buffer[:read]
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			log.Printf("cannot read from output: %v\n", err)
			return
		}
		p := api.Packet{}
		results := make([]struct {
			Id   int32
			Data []byte
		}, 0, 32)
		for len(data) > 0 {
			p.Reset()
			if err := proto.Unmarshal(data, &p); err != nil { // unmarshals the last protobuf object in the array
				log.Printf("cannot unmarshal: %v\n", err)
				break
			}

			size := proto.Size(&p) // calculate how much data to cut off for the next unmarshal
			data = data[:len(data)-size]

			results = append(results, struct {
				Id   int32
				Data []byte
			}{Id: p.Id, Data: p.Data}) // add a packet to results
		}
		for i := len(results) - 1; i >= 0; i-- { // write data in reverse to appropriate streams
			p := results[i]
			//fmt.Println(m.Name, p.Id, string(p.Data))

			for _, stream := range m.client.getStreams(p.Id) {
				if _, err := stream.input.Write(p.Data); err != nil {
					log.Printf("cannot write to stream input: %v\n", err)
					if err := m.closeStream(stream); err != nil { // close the stream if write unsuccessful
						m.logf("cannot close a stream %d: %v", stream.id, err)
					}
					break
				}
			}
		}
	}
}

// Creates a new stream for the connection.
// All incoming packets for the id will be passed to this stream.
func (m *mux) NewStream(id int32) *stream {
	//m.logf("created a new stream %d", id)
	str := &stream{
		client: m,
		id:     id,
		output: m.conn,
		input:  newBlockingReader(), // we used a byte buffer here before, but it's a non blocking read which doesn't suit us
	}
	m.client.addStream(id, str)
	m.streamMutex.Lock()
	defer m.streamMutex.Unlock()
	m.ownedStreams[str] = true
	return str
}

// Removes the stream from the mux, but doesn't close it. It also doesn't remove it from the client.
func (m *mux) removeStream(stream *stream) {
	m.streamMutex.Lock()
	defer m.streamMutex.Unlock()
	delete(m.ownedStreams, stream)
}

// Closes a stream and removes it from the client and the mux.
func (m *mux) closeStream(s *stream) error {
	m.removeStream(s)
	m.client.removeStream(s.id, s)
	return s.input.Close()
}

// Closes a mux and the streams it owns.
func (m *mux) Close() error {
	m.streamMutex.RLock()
	defer m.streamMutex.RUnlock()
	var resultErr error
	for s := range m.ownedStreams {
		if err := m.closeStream(s); err != nil {
			resultErr = err
		}
	}
	return resultErr
}
