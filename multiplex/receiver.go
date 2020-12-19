package multiplex

import (
	"fmt"
	"io"
	"log"
)

// Receiver is an individual data stream that only supports reading
type Receiver struct {
	client *Client            // Client responsible for this Receiver
	id     string             // Receiver ID
	input  io.ReadWriteCloser // Receiver input - receives data from Muxes
}

func (r *Receiver) ID() string {
	return r.id
}

func (r *Receiver) Read(p []byte) (n int, err error) {
	n, err = r.input.Read(p)
	log.Printf("receiver %s reading %s", r.id, string(p[:n]))
	return n, err
}

func (r *Receiver) WriteInput(bytes []byte) (n int, err error) {
	return r.input.Write(bytes)
}

func (r *Receiver) String() string {
	return fmt.Sprintf("Receiver(client: %s, id: %s, \n\tinput: %s, \n\t)", r.client.Name, r.id, r.input)
}

func (r *Receiver) Close() error {
	log.Printf("closed receiver %s", r)
	return r.input.Close()
}
