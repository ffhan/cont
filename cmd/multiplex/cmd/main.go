package main

import (
	"bufio"
	"bytes"
	"cont/cmd/multiplex"
	"fmt"
	"log"
	"net"
	"time"
)

func client() {
	time.Sleep(100 * time.Millisecond)
	dial, err := net.Dial("tcp", ":10000")
	if err != nil {
		panic(err)
	}
	mux := multiplex.NewMux(dial)
	mux.Name = "client"

	go func() {
		s1 := mux.NewSession(1)
		if _, err := fmt.Fprintln(s1, "hello from session 1!"); err != nil {
			panic(err)
		}
		scanner := bufio.NewScanner(s1)
		for scanner.Scan() {
			log.Printf("%s:%d -> %s\n", mux.Name, 1, scanner.Text())
		}
	}()

	go func() {
		s2 := mux.NewSession(2)
		if _, err := fmt.Fprintln(s2, "hello from session 2!"); err != nil {
			panic(err)
		}
		scanner := bufio.NewScanner(s2)
		for scanner.Scan() {
			log.Printf("%s:%d -> %s\n", mux.Name, 2, scanner.Text())
		}
	}()
}

type nopBuffer struct {
	bytes.Buffer
}

func (n *nopBuffer) Close() error {
	return nil
}

func main() {
	listen, err := net.Listen("tcp", ":10000")
	if err != nil {
		panic(err)
	}

	go client()

	for {
		accept, err := listen.Accept()
		if err != nil {
			panic(err)
		}
		go func() {
			mux := multiplex.NewMux(accept)
			mux.Name = "server"

			process1 := mux.NewSession(1)
			process2 := mux.NewSession(2)

			go func() {
				if _, err := fmt.Fprintln(process1, "example of process 1 writing"); err != nil {
					panic(err)
				}
				if _, err := fmt.Fprintln(process1, "example of process 1 writing... again!"); err != nil {
					panic(err)
				}
				scanner := bufio.NewScanner(process1)
				for scanner.Scan() {
					log.Printf("%s:%d -> %s\n", mux.Name, 1, scanner.Text())
				}
			}()
			go func() {
				if _, err := fmt.Fprintln(process2, "example of process 2 writing"); err != nil {
					panic(err)
				}
				if _, err := fmt.Fprintln(process2, "example of process 2 writing... again!"); err != nil {
					panic(err)
				}
				scanner := bufio.NewScanner(process2)
				for scanner.Scan() {
					log.Printf("%s:%d -> %s\n", mux.Name, 2, scanner.Text())
				}
			}()
		}()
	}
}
