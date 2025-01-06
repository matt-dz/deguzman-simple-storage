.PHONY: all

include .env
export

all:
	go run cmd/main.go

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/dss cmd/main.go

docker:
	docker build -t dss .
	docker tag dss $(DOCKER_REGISTRY)/dss
	docker push $(DOCKER_REGISTRY)/dss

hash-all:
	go run scripts/hash.go

clean:
	rm bin/*
