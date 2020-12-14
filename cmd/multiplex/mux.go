package multiplex

import (
	"cont/api"
	"errors"
	"github.com/golang/protobuf/proto"
	"io"
	"log"
)

type session struct {
	client *mux
	id     int32
	conn   io.ReadWriter
	line   io.ReadWriter
}

func (s *session) Read(p []byte) (n int, err error) {
	return s.line.Read(p)
}

func (s *session) Write(p []byte) (n int, err error) {
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

type mux struct {
	Name    string
	conn    io.ReadWriteCloser
	streams map[int32]*session
}

func NewMux(conn io.ReadWriteCloser) *mux {
	m := &mux{
		conn:    conn,
		streams: make(map[int32]*session),
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

			stream, ok := m.streams[p.Id] // this assumes 1:1 multiplexing, we'd like M:N (just store an array of streams)
			if !ok {
				log.Printf("no stream with id %d\n", p.Id)
				break
			}
			if _, err := stream.line.Write(p.Data); err != nil {
				log.Printf("cannot write to stream line: %v\n", err)
				m.closeSession(p.Id)
				break
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

func (m *mux) NewSession(id int32) *session {
	sesh := &session{
		client: m,
		id:     id,
		conn:   m.conn,
		line:   newBlockingReader(), // we used a byte buffer here before, but it's a non blocking read which doesn't suit us
	}
	m.streams[id] = sesh
	return sesh
}

func (m *mux) closeSession(id int32) {
	delete(m.streams, id)
}

func (m *mux) Close() error {
	for id := range m.streams {
		m.closeSession(id)
	}
	return nil
}
