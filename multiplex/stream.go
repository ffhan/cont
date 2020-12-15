package multiplex

import (
	"cont/api"
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

func (s *Stream) Read(p []byte) (n int, err error) {
	return s.input.Read(p)
}

func (s *Stream) Write(p []byte) (n int, err error) {
	payload, err := proto.Marshal(&api.Packet{
		Id:   s.id,
		Data: p,
	})
	if err != nil {
		return 0, err
	}
	_, err = s.output.Write(payload)
	return len(p), err
}

func (s *Stream) Close() error {
	return s.input.Close()
}
