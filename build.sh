#!/bin/sh

GOOS=linux GOARCH=amd64 go build -o web.linux

GOOS=windows GOARCH=amd64 go build -o web.exe

GOOS=darwin GOARCH=amd64 go build -o web.mac

