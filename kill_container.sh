#!/bin/bash

read -r container_id

echo "go run cmd/cli/cli.go kill --host 127.0.0.1 $container_id"
go run cmd/cli/cli.go kill --host 127.0.0.1 $container_id
