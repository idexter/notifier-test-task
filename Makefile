.PHONY: test docs build

all: lint build test

lint:
	golangci-lint run

build:
	go build ./pkg/notifier
	go build -o ./build -i ./cmd/notify
	go build -o ./build -i ./cmd/notify-test-server

test:
	go test -cover -v -race ./...

coverage:
	go test -coverprofile cover.out ./...
	go tool cover -html=cover.out

docs:
	@echo "Docs available at: http://localhost:8888/pkg/github.com/idexter/notifier-test-task/"
	godoc -http :8888

ci-coverage-dependencies:
	go get github.com/axw/gocov/...
	go get github.com/AlekSi/gocov-xml
	go mod tidy

ci-coverage-report: ci-coverage-dependencies
	go test -race -covermode=atomic -coverprofile=coverage.txt ./...
	gocov convert coverage.txt | gocov-xml > coverage.xml

clean:
	rm -f ./build/notify
	rm -f ./build/notify-test-server
	rm -f trace.out
	rm -f cover.out
	rm -f ./coverage.txt
	rm -f ./coverage.xml
