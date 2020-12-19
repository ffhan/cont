#!/bin/bash

read -r container_id

echo "go run cmd/cli/cli.go attach --host 127.0.0.1 --it $container_id"
go run cmd/cli/cli.go attach --host 127.0.0.1 --it $container_id