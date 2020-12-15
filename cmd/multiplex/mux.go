package multiplex

import (
	"cont/api"
	"errors"
	"github.com/golang/protobuf/proto"
	"io"
	"log"
	"sync"
)

type stream struct {
	client *mux
	id     int32
	conn   io.ReadWriter
	line   io.ReadWriteCloser
}

func (s *stream) Read(p []byte) (n int, err error) {
	return s.line.Read(p)
}

func (s *stream) Write(p []byte) (n int, err error) {
	payload, err := proto.Marshal(&api.Packet{
		Id:   s.id,
		Data: p,
	})
	if err != nil {
		return 0, err
	}
	_, err = s.conn.Write(payload)
	return len(p), err
}

func (s *stream) Close() error {
	return s.line.Close()
}

type client struct {
	streams     map[int32]map[*stream]bool
	streamMutex sync.RWMutex
	Name        string
}

func NewClient() *client {
	return &client{streams: make(map[int32]map[*stream]bool)}
}

func (c *client) Close() error {
	for id, streams := range c.streams {
		for stream := range streams {
			err := stream.Close()
			if err != nil {
				log.Println(err)
			}
			c.removeStream(id, stream)
		}
	}
	return nil
}

func (c *client) log(format string, args ...interface{}) {
	log.Printf("%s:"+format, append([]interface{}{c.Name}, args...)...)
}

func (c *client) addStream(id int32, str *stream) {
	//c.log("added stream %d", id)
	c.streamMutex.Lock()
	defer c.streamMutex.Unlock()
	if _, ok := c.streams[id]; !ok {
		c.streams[id] = make(map[*stream]bool)
	}
	c.streams[id][str] = true
}

func (c *client) removeStream(id int32, stream *stream) {
	//c.log("removed stream %d", id)
	c.streamMutex.Lock()
	defer c.streamMutex.Unlock()
	if _, ok := c.streams[id]; !ok {
		return
	}
	delete(c.streams[id], stream)
}

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

type mux struct {
	client *client
	Name   string
	conn   io.ReadWriteCloser
}

func (m *mux) log(format string, args ...interface{}) {
	m.client.log("%s: "+format, append([]interface{}{m.Name}, args...)...)
}

func (c *client) NewMux(conn io.ReadWriteCloser) *mux {
	//c.log("created a new mux\n")
	m := &mux{
		client: c,
		conn:   conn,
	}
	go m.readIncoming()
	return m
}

func (m *mux) readIncoming() {
	buffer := make([]byte, 2048)
	for {
		read, err := m.conn.Read(buffer)
		data := buffer[:read]
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			log.Printf("cannot read from conn: %v\n", err)
			return
		}
		p := api.Packet{}
		results := make([]struct {
			Id   int32
			Data []byte
		}, 0, 32)
		for len(data) > 0 {
			p.Reset()
			if err := proto.Unmarshal(data, &p); err != nil {
				log.Printf("cannot unmarshal: %v\n", err)
				break
			}

			size := proto.Size(&p)
			data = data[:len(data)-size]

			results = append(results, struct {
				Id   int32
				Data []byte
			}{Id: p.Id, Data: p.Data})
		}
		for i := len(results) - 1; i >= 0; i-- {
			p := results[i]
			//fmt.Println(m.Name, p.Id, string(p.Data))

			for _, stream := range m.client.getStreams(p.Id) {
				if _, err := stream.line.Write(p.Data); err != nil {
					log.Printf("cannot write to stream line: %v\n", err)
					m.closeSession(p.Id)
					break
				}
			}
		}
	}
}

type blockingReader struct {
	reader io.ReadCloser
	writer io.WriteCloser
}

func newBlockingReader() *blockingReader {
	reader, writer := io.Pipe()
	return &blockingReader{reader: reader, writer: writer}
}

func (b *blockingReader) Read(p []byte) (n int, err error) {
	return b.reader.Read(p)
}

func (b *blockingReader) Write(p []byte) (n int, err error) {
	return b.writer.Write(p)
}

func (b *blockingReader) Close() error {
	err := b.writer.Close()
	err2 := b.reader.Close()
	if err != nil {
		return err
	}
	return err2
}

func (m *mux) NewStream(id int32) *stream {
	//m.log("created a new stream %d", id)
	sesh := &stream{
		client: m,
		id:     id,
		conn:   m.conn,
		line:   newBlockingReader(), // we used a byte buffer here before, but it's a non blocking read which doesn't suit us
	}
	m.client.addStream(id, sesh)
	return sesh
}

func (m *mux) closeSession(id int32) {
	for _, stream := range m.client.getStreams(id) {
		if err := stream.line.Close(); err != nil {
			log.Printf("canont close stream line: %v\n", err)
		}
		m.client.removeStream(id, stream)
	}
}

func (m *mux) Close() error {
	return m.conn.Close()
}
