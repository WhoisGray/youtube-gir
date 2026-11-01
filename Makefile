.PHONY: test build run docker

test:
	go test ./...

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o dist/youtubegir ./cmd/youtubegir

run:
	go run ./cmd/youtubegir

docker:
	docker compose up -d --build
