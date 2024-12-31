.PHONY: all

include .env
export

all:
	go run src/main.go

build:
	go build -o bin/dss src/main.go

clean:
	rm bin/*
