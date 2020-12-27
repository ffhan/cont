package container

import (
	_ "cont/nsenter"
	"io"
)

const (
	initPipeEnv = "_LIBCONTAINER_INITPIPE"
)

type SharedNamespaceConfig struct {
	Share bool
	PID   int
}

type LoggingConfig struct {
	Path string
}

type Config struct {
	Stdin                 io.Reader
	Stdout, Stderr        io.Writer
	Hostname              string
	Workdir               string
	Cmd                   string
	Args                  []string
	Interactive           bool
	SharedNamespaceConfig SharedNamespaceConfig
	Logging               LoggingConfig
}

type initPipeConfig struct {
	Hostname, Workdir     string
	Interactive           bool
	SharedNamespaceConfig SharedNamespaceConfig
}
