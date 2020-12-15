package multiplex

import "io"

// io.ReadWriteCloser that blocks read until there's data to read
type blockingReadWriteCloser struct {
	reader io.ReadCloser
	writer io.WriteCloser
}

func newBlockingReader() *blockingReadWriteCloser {
	reader, writer := io.Pipe()
	return &blockingReadWriteCloser{reader: reader, writer: writer}
}

func (b *blockingReadWriteCloser) Read(p []byte) (n int, err error) {
	return b.reader.Read(p)
}

func (b *blockingReadWriteCloser) Write(p []byte) (n int, err error) {
	return b.writer.Write(p)
}

func (b *blockingReadWriteCloser) Close() error {
	err := b.writer.Close()
	err2 := b.reader.Close()
	if err != nil {
		return err
	}
	return err2
}
