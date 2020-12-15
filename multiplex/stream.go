package multiplex

import (
	"cont/api"
	"github.com/golang/protobuf/proto"
	"io"
)

// stream is an individual data stream, a part of a multiplexed stream
type stream struct {
	client *mux               // mux responsible for this stream
	id     int32              // stream ID
	output io.ReadWriter      // stream output - writes data out
	input  io.ReadWriteCloser // stream input - receives data from mux
}

func (s *stream) Read(p []byte) (n int, err error) {
	return s.input.Read(p)
}

func (s *stream) Write(p []byte) (n int, err error) {
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

func (s *stream) Close() error {
	return s.input.Close()
}
