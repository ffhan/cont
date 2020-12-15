package main

import (
	"fmt"
	"io"
)

type dynamicPipe struct {
	pipes       map[io.ReadWriteCloser]bool
	multiReader io.Reader
}

func NewDynamicPipe() *dynamicPipe {
	return &dynamicPipe{pipes: make(map[io.ReadWriteCloser]bool)}
}

func (d *dynamicPipe) Add(closer io.ReadWriteCloser) {
	d.pipes[closer] = true
	d.recreate()
}

func (d *dynamicPipe) Remove(closer io.ReadWriteCloser) {
	delete(d.pipes, closer)
	d.recreate()
}

func (d *dynamicPipe) recreate() {
	readers := make([]io.Reader, 0, len(d.pipes))
	for r := range d.pipes {
		readers = append(readers, r)
	}
	d.multiReader = io.MultiReader(readers...)
}

func (d *dynamicPipe) Read(p []byte) (n int, err error) {
	n, err = d.multiReader.Read(p)
	fmt.Println("dynamic read: ", string(p[:n]))
	return n, err
}

func (d *dynamicPipe) Write(p []byte) (n int, err error) {
	fmt.Println("dynamic write: ", string(p))
	for writer := range d.pipes {
		writer := writer
		go writer.Write(p)
	}
	return len(p), nil
}

func (d *dynamicPipe) Close() error {
	var resultErr error
	for closer := range d.pipes {
		if err := closer.Close(); err != nil {
			resultErr = err
		}
	}
	d.multiReader = nil
	d.pipes = nil
	return resultErr
}
