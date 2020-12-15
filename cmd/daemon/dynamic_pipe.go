package main

import (
	"container/ring"
	"io"
	"sync"
	"time"
)

const (
	totalBufferSize = 8192
	ringParts       = 64
	ringPieceBuffer = totalBufferSize / ringParts
)

type dynamicPipe struct {
	pipes      map[io.ReadWriteCloser]chan bool
	ringBuffer *ring.Ring
	ringMutex  sync.Mutex
	readRing   *ring.Ring
	readMutex  sync.RWMutex
	gotData    chan bool
	pipeMutex  sync.RWMutex
}

func NewDynamicPipe() *dynamicPipe {
	ringBuf := ring.New(ringParts)
	return &dynamicPipe{
		pipes:      make(map[io.ReadWriteCloser]chan bool),
		ringBuffer: ringBuf,
		readRing:   ringBuf,
		gotData:    make(chan bool, 1),
	}
}

func (d *dynamicPipe) Add(closer io.ReadWriteCloser) {
	d.pipeMutex.Lock()
	defer d.pipeMutex.Unlock()
	d.pipes[closer] = make(chan bool, 1)
	go d.bgRead(closer)
}

func (d *dynamicPipe) Remove(closer io.ReadWriteCloser) {
	d.pipeMutex.Lock()
	defer d.pipeMutex.Unlock()
	if stopChan, ok := d.pipes[closer]; ok {
		close(stopChan)
	}
	delete(d.pipes, closer)
}

func (d *dynamicPipe) getPipes() map[io.ReadWriteCloser]chan bool {
	d.pipeMutex.RLock()
	defer d.pipeMutex.RUnlock()
	return d.pipes
}

func (d *dynamicPipe) getPipe(r io.ReadWriteCloser) (chan bool, bool) {
	d.pipeMutex.RLock()
	defer d.pipeMutex.RUnlock()
	p, ok := d.pipes[r]
	return p, ok
}

func (d *dynamicPipe) bgRead(reader io.ReadWriteCloser) {
	defer func() {
		delete(d.pipes, reader)
	}()
	bytes := make([]byte, ringPieceBuffer)
	for {
		p, ok := d.getPipe(reader)
		if !ok {
			return
		} else {
			select {
			case <-p:
				return
			case <-time.After(10 * time.Microsecond):
			}
		}
		read, err := reader.Read(bytes)
		if err != nil {
			return
		}
		result := make([]byte, read)
		copy(result, bytes[:read])

		d.ringMutex.Lock()

		d.ringBuffer.Value = result
		d.ringBuffer = d.ringBuffer.Next()
		select {
		case d.gotData <- true:
		case <-time.After(10 * time.Microsecond): // there's gotta be a better way
		}

		d.ringMutex.Unlock()
	}
}

func (d *dynamicPipe) getValue() []byte {
	d.readMutex.RLock()
	defer d.readMutex.RUnlock()
	if d.readRing.Value == nil {
		return nil
	}
	return d.readRing.Value.([]byte)
}

func (d *dynamicPipe) nextRead() {
	d.readMutex.Lock()
	defer d.readMutex.Unlock()
	d.readRing = d.readRing.Next()
}

func (d *dynamicPipe) updateCurrentRead(b []byte) {
	d.readMutex.Lock()
	d.readMutex.Unlock()
	d.readRing.Value = b
}

func (d *dynamicPipe) nextWrite() {
	d.ringMutex.Lock()
	defer d.ringMutex.Unlock()
	d.ringBuffer = d.ringBuffer.Next()
}

func (d *dynamicPipe) Read(p []byte) (n int, err error) {
	for len(d.getPipes()) > 0 || d.getValue() != nil {
		bytes := d.getValue()
		if bytes == nil {
			select {
			case <-d.gotData:
			case <-time.After(10 * time.Millisecond): // enforce rechecking
				continue
			}
			continue
		}
		if len(bytes) <= len(p) {
			copy(p, bytes)
			d.nextRead()
			n = len(bytes)
			err = nil
			break
		} else {
			// cut off the buffer, don't advance it
			copy(p, bytes)
			d.updateCurrentRead(bytes[len(p):])
			n = len(p)
			err = nil
			break
		}
	}
	if len(d.getPipes()) == 0 && d.getValue() == nil {
		return 0, io.EOF // we have exhausted all pipes
	}
	return n, err
}

func (d *dynamicPipe) Write(p []byte) (n int, err error) {
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
	d.pipes = nil
	return resultErr
}
