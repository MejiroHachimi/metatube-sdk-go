.PHONY: build run test verify
build:
	CGO_ENABLED=0 go build -trimpath -ldflags '-s -w -X github.com/metatube-community/metatube-sdk-go/internal/version.Version=0.1.0' -o build/metatube ./cmd/metatube
run:
	go run ./cmd/metatube
test:
	go test -race -coverprofile=coverage.out ./internal/service ./imageutil ./database ./engine/dbengine ./engine
	go test -race ./provider/av-league ./provider/theporndb -run TestFixture
verify: test
	go build ./...
