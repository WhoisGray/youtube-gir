.PHONY: test build run docker package gui

test:
	go test ./...

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o dist/youtubegir ./cmd/youtubegir
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o dist/youtubegir-gui ./cmd/youtubegir-gui

package:
	VERSION=$${VERSION:-dev} bash scripts/package.sh

run:
	go run ./cmd/youtubegir

gui:
	go run ./cmd/youtubegir-gui

docker:
	docker compose up -d --build
