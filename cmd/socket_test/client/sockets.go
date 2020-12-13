package main

import (
	"cont/cmd/socket_test"
	"net"
)

func main() {
	dial, err := net.Dial("unix", "/tmp/test.sock")
	socket_test.Must(err)
	socket_test.HandleConnection(dial)
}
