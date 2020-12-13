package main

import (
	"cont/cmd/socket_test"
	"fmt"
	"net"
)

func main() {
	listen, err := net.Listen("unix", "/tmp/test.sock")
	socket_test.Must(err)
	defer listen.Close()

	for {
		conn, err := listen.Accept()
		fmt.Println("accepted a connection!")
		if err != nil {
			panic(err)
		}
		go socket_test.HandleConnection(conn)
	}
}
