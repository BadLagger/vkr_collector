.PHONY: all clean build-collector

all: build-collector

build-collector:
	GOOS=linux GOARCH=arm64 go build -o bin/collector main.go

clean:
	rm -rf bin/