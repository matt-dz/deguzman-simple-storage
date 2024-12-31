.PHONY: all

include .env
export

all:
	go run cmd/main.go

build:
	go build -o bin/dss cmd/main.go

clean:
	rm bin/*
