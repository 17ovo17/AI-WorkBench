#!/bin/bash
cd "$(dirname "$0")/../api"
go mod tidy
go run main.go
