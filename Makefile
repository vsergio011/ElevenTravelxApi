APP_PATH=./cmd/api

.PHONY: fmt test tidy run

fmt:
	gofmt -w ./cmd ./internal

test:
	go test ./...

tidy:
	go mod tidy

run:
	go run $(APP_PATH)