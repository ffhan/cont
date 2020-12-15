package multiplex

import (
	"cont/api"
	"fmt"
	"github.com/golang/protobuf/proto"
	"io"
)

// Stream is an individual data Stream, a part of a multiplexed Stream
type Stream struct {
	client *Mux               // Mux responsible for this Stream
	id     string             // Stream ID
	output io.ReadWriter      // Stream output - writes data out
	input  io.ReadWriteCloser // Stream input - receives data from Mux
}

func (s *Stream) String() string {
	return fmt.Sprintf("Stream(client: %s, mux: %s, id: %s)", s.client.client.Name, s.client.Name, s.id)
}

func (s *Stream) Read(p []byte) (n int, err error) {
	n, err = s.input.Read(p)
	fmt.Println(s.id, " read: ", string(p[:n]))
	return n, err
}

func (s *Stream) Write(p []byte) (n int, err error) {
	fmt.Println(s.id, " written: ", string(p))
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

func (s *Stream) Close() error {
	return s.input.Close()
}
