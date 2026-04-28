BIN := bin/onvif-server
PKG := ./cmd/onvif-server

.PHONY: build run test coverage vet tidy clean docker

build:
	mkdir -p bin
	go build -o $(BIN) $(PKG)

run:
	go run $(PKG) -config config.yaml

test:
	go test ./...

coverage:
	go test -race -covermode=atomic -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -n 1
	@echo "HTML report: run 'go tool cover -html=coverage.out'"

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -rf bin coverage.out

docker:
	docker build -t onvif-server .
