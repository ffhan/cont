package socket_test

import (
	"bufio"
	"fmt"
	"net"
	"os"
)

func HandleConnection(conn net.Conn) {
	go handleWrite(conn)
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		text := scanner.Text()
		fmt.Println("response: ", text)
	}
}

func handleWrite(conn net.Conn) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		bytes := scanner.Text()
		fmt.Println(bytes)
		n, err := conn.Write([]byte(bytes + "\n"))
		fmt.Printf("sent %d bytes to the socket\n", n)
		Must(err)
	}
}

func Must(err error) {
	if err != nil {
		panic(err)
	}
}
