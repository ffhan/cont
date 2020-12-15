package main

import (
	"bytes"
	"cont/multiplex"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"sync"
	"time"
)

var mutex sync.Mutex

func client() {
	multiplexClient := multiplex.NewClient()
	multiplexClient.Name = "client"

	time.Sleep(100 * time.Millisecond)
	dial, err := net.Dial("tcp", ":10000")
	if err != nil {
		panic(err)
	}
	mux := multiplexClient.NewMux(dial)
	mux.Name = "mux1"

	go func() {
		s1 := mux.NewStream(1)
		if _, err := fmt.Fprintln(s1, "hello from session 1!"); err != nil {
			panic(err)
		}
		var buf bytes.Buffer
		go io.Copy(&buf, s1)
		time.Sleep(200 * time.Millisecond)
		mutex.Lock()
		defer mutex.Unlock()
		log.Println("---------------------------")
		log.Printf("%s:%s:%d\n", multiplexClient.Name, mux.Name, 1)
		log.Println(buf.String())
	}()

	go func() {
		conn, err := net.Dial("tcp", ":10000")
		if err != nil {
			panic(err)
		}
		mux2 := multiplexClient.NewMux(conn)
		mux2.Name = "mux2"
		s22 := mux2.NewStream(2)
		if _, err := fmt.Fprintln(s22, "hello from session 2 - copy!"); err != nil {
			panic(err)
		}
		var buf bytes.Buffer
		go io.Copy(&buf, s22)
		time.Sleep(200 * time.Millisecond)
		mutex.Lock()
		defer mutex.Unlock()
		log.Println("---------------------------")
		log.Printf("%s:%s:%d\n", multiplexClient.Name, mux2.Name, 2)
		log.Println(buf.String())
	}()

	go func() {
		s2 := mux.NewStream(2)
		if _, err := fmt.Fprintln(s2, "hello from session 2!"); err != nil {
			panic(err)
		}
		var buf bytes.Buffer
		go io.Copy(&buf, s2)
		time.Sleep(200 * time.Millisecond)
		mutex.Lock()
		defer mutex.Unlock()
		log.Println("---------------------------")
		log.Printf("%s:%s:%d\n", multiplexClient.Name, mux.Name, 2)
		log.Println(buf.String())
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

	multiplexClient := multiplex.NewClient()
	multiplexClient.Name = "server"

	counter := 0
	for {
		accept, err := listen.Accept()
		if err != nil {
			panic(err)
		}
		counter += 1
		go func(counter int) {
			mux := multiplexClient.NewMux(accept)
			mux.Name = "mux" + strconv.Itoa(counter)

			process1 := mux.NewStream(1)
			process2 := mux.NewStream(2)

			go func() {
				if _, err := fmt.Fprintln(process1, strconv.Itoa(counter)+": example of process 1 writing"); err != nil {
					panic(err)
				}
				if _, err := fmt.Fprintln(process1, strconv.Itoa(counter)+": example of process 1 writing... again!"); err != nil {
					panic(err)
				}
				var buf bytes.Buffer
				go io.Copy(&buf, process1)
				time.Sleep(200 * time.Millisecond)
				mutex.Lock()
				defer mutex.Unlock()
				log.Println("---------------------------")
				log.Printf("%s:%s:%d\n", multiplexClient.Name, mux.Name, 1)
				log.Println(buf.String())
			}()
			go func() {
				if _, err := fmt.Fprintln(process2, strconv.Itoa(counter)+": example of process 2 writing"); err != nil {
					panic(err)
				}
				if _, err := fmt.Fprintln(process2, strconv.Itoa(counter)+": example of process 2 writing... again!"); err != nil {
					panic(err)
				}
				var buf bytes.Buffer
				go io.Copy(&buf, process2)
				time.Sleep(200 * time.Millisecond)
				mutex.Lock()
				defer mutex.Unlock()
				log.Println("---------------------------")
				log.Printf("%s:%s:%d\n", multiplexClient.Name, mux.Name, 2)
				log.Println(buf.String())
			}()
		}(counter)
	}
}
