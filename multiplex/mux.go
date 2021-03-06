package multiplex

import (
	"cont/api"
	"errors"
	"github.com/golang/protobuf/proto"
	"io"
	"log"
	"sync"
)

// Mux multiplexes a connection to a number of streams.
// Every Mux handles only one connection.
type Mux struct {
	ownedStreams map[Streamer]bool
	streamMutex  sync.RWMutex
	client       *Client
	Name         string
	conn         io.ReadWriteCloser
	onClose      []func(mux *Mux)
}

func (m *Mux) AddOnClose(onClose func(mux *Mux)) {
	m.onClose = append(m.onClose, onClose)
}

func (m *Mux) GetOwnedStreams() map[Streamer]bool {
	return m.ownedStreams
}

func (m *Mux) logf(format string, args ...interface{}) {
	m.client.logf("%s: "+format, append([]interface{}{m.Name}, args...)...)
}

const maxBuffer = 32768

// read incoming data from a connection and pass it to appropriate streams
func (m *Mux) readIncoming() {
	defer m.Close()
	buffer := make([]byte, maxBuffer)
	for {
		//fmt.Println("starting reading in mux")
		read, err := m.conn.Read(buffer)
		data := buffer[:read]
		if err != nil {
			if errors.Is(err, io.EOF) {
				// EOF should be silent
				//log.Printf("cannot read from output (EOF): %v\n", err)
				return
			}
			log.Printf("cannot read from output: %v\n", err)
			return
		}
		p := api.Packet{}
		results := make([]struct {
			Id   string
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
				Id   string
				Data []byte
			}{Id: p.Id, Data: p.Data}) // add a packet to results
		}
		for i := len(results) - 1; i >= 0; i-- { // write data in reverse to appropriate streams
			p := results[i]
			//log.Println(m.Name, p.Id, string(p.Data))

			streams := m.client.getStreams(p.Id)
			var wg sync.WaitGroup
			wg.Add(len(streams))

			//fmt.Println("streams: ", streams)
			for _, stream := range streams {
				stream := stream
				go func() {
					defer wg.Done()
					//log.Printf("mux sending %s to stream %s input", string(p.Data), stream)
					if _, err := stream.WriteInput(p.Data); err != nil {
						log.Printf("cannot write to Stream input: %v\n", err)
						if err := m.closeStream(stream); err != nil { // close the Stream if write unsuccessful
							m.logf("cannot close a Stream %s: %v", stream.ID(), err)
						}
						//} else {
						//log.Printf("%s sent to stream %s", string(p.Data), stream)
					}
				}()
			}
			wg.Wait()
		}
	}
}

func (m *Mux) write(id string, p []byte) (n int, err error) {
	//log.Printf("stream %s writing %s", id, string(p))
	payload, err := proto.Marshal(&api.Packet{
		Id:   id,
		Data: p,
	})
	if err != nil {
		return 0, err
	}
	idx := 0
	if len(payload) <= maxBuffer {
		_, err := m.conn.Write(payload)
		return len(p), err
	}
	packet := payload[maxBuffer*idx : maxBuffer*(idx+1)]
	for len(packet) > 0 {
		_, err = m.conn.Write(packet)
		if err != nil {
			log.Printf("cannot write to stream output: %v", err)
			return 0, err
		}
		idx += 1
		start := maxBuffer * idx
		end := maxBuffer * (idx + 1)
		if start >= len(payload) {
			break
		}
		if end >= len(payload) {
			end = len(payload) - 1
		}
		packet = payload[start:end]
	}
	return len(p), err
}

// Creates a new Stream for the connection.
// All incoming packets for the id will be passed to this Stream.
func (m *Mux) NewStream(id string) *Stream {
	//m.logf("created a new Stream %s", id)
	str := &Stream{
		mux:    m,
		id:     id,
		output: m.conn,
		input:  NewBlockingReader(), // we used a byte buffer here before, but it's a non blocking read which doesn't suit us
	}
	m.client.addStream(id, str)
	m.streamMutex.Lock()
	defer m.streamMutex.Unlock()
	m.ownedStreams[str] = true
	return str
}

// Removes the Stream from the Mux, but doesn't close it. It also doesn't remove it from the Client.
func (m *Mux) removeStream(stream Streamer) {
	m.streamMutex.Lock()
	defer m.streamMutex.Unlock()
	delete(m.ownedStreams, stream)
	m.client.removeStream(stream.ID(), stream)
}

// Closes a Stream and removes it from the Client and the Mux.
func (m *Mux) closeStream(s Streamer) error {
	//log.Printf("closed stream %s", s)
	m.removeStream(s)
	m.client.removeStream(s.ID(), s)
	return s.Close()
}

func (m *Mux) executeOnClose() {
	//log.Println("executing OnClose mux functions")
	for _, f := range m.onClose {
		f := f
		go f(m)
	}
}

// Closes a Mux and the streams it owns.
func (m *Mux) Close() error {
	//log.Println("closed mux")
	defer m.executeOnClose()

	m.client.muxMutex.Lock()
	delete(m.client.muxes, m)
	m.client.muxMutex.Unlock()

	var resultErr error
	for s := range m.ownedStreams {
		if err := m.closeStream(s); err != nil {
			resultErr = err
		}
	}
	return resultErr
}
