BIN := bin/onvif-server
PKG := ./cmd/onvif-server

.PHONY: build run test vet tidy clean docker

build:
	mkdir -p bin
	go build -o $(BIN) $(PKG)

run:
	go run $(PKG) -config config.yaml

test:
	go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -rf bin

docker:
	docker build -t onvif-server .
