APP=s3back
VERSION?=0.1.0

build:
	go build -ldflags "-X main.Version=$(VERSION)" -o bin/$(APP) ./cmd/s3back

run: build
	./bin/$(APP)

test:
	go test -race -count=1 ./...

integration-test:
	go test -tags=integration -count=1 ./...

lint:
	go vet ./...

fmt:
	gofmt -s -w .

clean:
	rm -rf bin dist

release: clean
	mkdir -p dist
	GOOS=linux  GOARCH=amd64 go build -ldflags "-X main.Version=$(VERSION)" -o dist/$(APP)-linux-amd64  ./cmd/s3back
	GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.Version=$(VERSION)" -o dist/$(APP)-darwin-amd64 ./cmd/s3back
	GOOS=darwin GOARCH=arm64 go build -ldflags "-X main.Version=$(VERSION)" -o dist/$(APP)-darwin-arm64 ./cmd/s3back

.PHONY: build run test integration-test lint fmt clean release
