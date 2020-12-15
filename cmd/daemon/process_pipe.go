package main

import (
	"io"
	"sync"
)

type dynamicReader struct {
	readers []io.ReadCloser
	mutex   sync.RWMutex

	in io.Reader
}

func NewDynamicReader() *dynamicReader {
	return &dynamicReader{
		readers: make([]io.ReadCloser, 0, 4),
	}
}

func (d *dynamicReader) AddReader(closer io.ReadCloser) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.readers = append(d.readers, closer)
	d.recreateIn()
}

func (d *dynamicReader) recreateIn() {
	readers := make([]io.Reader, len(d.readers))
	for i, r := range d.readers {
		readers[i] = r
	}
	d.in = io.MultiReader(readers...)
}

func (d *dynamicReader) RemoveReader(closer io.ReadCloser) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	for i, reader := range d.readers {
		if reader == closer {
			d.readers = append(d.readers[:i], d.readers[i+1:]...)
			return
		}
	}
}

func (d *dynamicReader) Read(p []byte) (n int, err error) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	if d.in == nil {
		return 0, io.EOF
	}
	return d.in.Read(p)
}

func (d *dynamicReader) Close() error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	var resultErr error
	for _, reader := range d.readers {
		err := reader.Close()
		if err != nil {
			resultErr = err
		}
	}
	d.in = nil
	d.readers = d.readers[0:0]
	return resultErr
}

type dynamicWriter struct {
	writers []io.WriteCloser
	mutex   sync.RWMutex

	out io.Writer
}

func NewDynamicWriter() *dynamicWriter {
	return &dynamicWriter{writers: make([]io.WriteCloser, 0, 4)}
}

func (d *dynamicWriter) Write(p []byte) (n int, err error) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	if d.out == nil {
		return 0, io.EOF // todo: check if this should happen
	}
	return d.out.Write(p)
}

func (d *dynamicWriter) AddWriter(closer io.WriteCloser) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.writers = append(d.writers, closer)
	d.recreateIn()
}

func (d *dynamicWriter) recreateIn() {
	readers := make([]io.Writer, len(d.writers))
	for i, r := range d.writers {
		readers[i] = r
	}
	d.out = io.MultiWriter(readers...)
}

func (d *dynamicWriter) RemoveWriter(closer io.WriteCloser) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	for i, reader := range d.writers {
		if reader == closer {
			d.writers = append(d.writers[:i], d.writers[i+1:]...)
			return
		}
	}
}

func (d *dynamicWriter) Close() error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	var resultErr error
	for _, reader := range d.writers {
		err := reader.Close()
		if err != nil {
			resultErr = err
		}
	}
	d.out = nil
	d.writers = d.writers[0:0]
	return resultErr
}
