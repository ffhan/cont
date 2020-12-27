#!/bin/bash

read -r container_id

go run ./cmd/cli/cli.go run --host 127.0.0.1 --share-ns "$container_id" --it --name shared bash
