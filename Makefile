.PHONY: test build run docker package

test:
	go test ./...

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o dist/youtubegir ./cmd/youtubegir

package:
	VERSION=$${VERSION:-dev} bash scripts/package.sh

run:
	go run ./cmd/youtubegir

docker:
	docker compose up -d --build
