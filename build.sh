#!/bin/sh
go get github.com/gorilla/mux
go get github.com/sirupsen/logrus

GOOS=linux GOARCH=amd64 go build -o go_web/web.linux

GOOS=windows GOARCH=amd64 go build -o go_web/web.exe

GOOS=darwin GOARCH=amd64 go build -o go_web/web.mac

