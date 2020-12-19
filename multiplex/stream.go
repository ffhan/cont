package multiplex

import (
	"fmt"
	"io"
)

// Stream is an individual data Stream, a part of a multiplexed Stream
type Stream struct {
	mux    *Mux               // Mux responsible for this Stream
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
	//log.Printf("stream %s reading %s", s.id, string(p[:n]))
	return n, err
}

func (s *Stream) Write(p []byte) (n int, err error) {
	return s.mux.write(s.id, p)
}

func (s *Stream) String() string {
	return fmt.Sprintf("Stream(mux: %s, mux: %s, id: %s, \n\tinput: %s, \n\toutput: %s)", s.mux.client.Name, s.mux.Name, s.id, s.input, s.output)
}

func (s *Stream) Close() error {
	//log.Printf("closed stream %s", s)
	return s.input.Close()
}
