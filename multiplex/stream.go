package multiplex

import (
	"cont/api"
	"fmt"
	"github.com/golang/protobuf/proto"
	"io"
	"log"
)

// Stream is an individual data Stream, a part of a multiplexed Stream
type Stream struct {
	client *Mux               // Mux responsible for this Stream
	id     string             // Stream ID
	output io.ReadWriter      // Stream output - writes data out
	input  io.ReadWriteCloser // Stream input - receives data from Mux
}

func (s *Stream) ID() string {
	return s.id
}

func (s *Stream) WriteInput(bytes []byte) (n int, err error) {
	return s.input.Write(bytes)
}

func (s *Stream) Read(p []byte) (n int, err error) {
	n, err = s.input.Read(p)
	log.Printf("stream %s reading %s", s.id, string(p[:n]))
	return n, err
}

func (s *Stream) Write(p []byte) (n int, err error) {
	log.Printf("stream %s writing %s", s.id, string(p))
	payload, err := proto.Marshal(&api.Packet{
		Id:   s.id,
		Data: p,
	})
	if err != nil {
		return 0, err
	}
	idx := 0
	if len(payload) <= maxBuffer {
		_, err := s.output.Write(payload)
		return len(p), err
	}
	packet := payload[maxBuffer*idx : maxBuffer*(idx+1)]
	for len(packet) > 0 {
		_, err = s.output.Write(packet)
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

func (s *Stream) String() string {
	return fmt.Sprintf("Stream(client: %s, mux: %s, id: %s, \n\tinput: %s, \n\toutput: %s)", s.client.client.Name, s.client.Name, s.id, s.input, s.output)
}

func (s *Stream) Close() error {
	log.Printf("closed stream %s", s)
	return s.input.Close()
}
