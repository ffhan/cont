#!/bin/bash

echo "go run cmd/cli/cli.go run --host 127.0.0.1 --workdir /home/fhancic --hostname test2 --it bash"
go run cmd/cli/cli.go run --host 127.0.0.1 --workdir /home/fhancic --hostname test2 --it bash
