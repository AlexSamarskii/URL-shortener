.PHONY: build run test clean docker-build docker-run migrate

BINARY_NAME=url_shortener

CMD_PATH=./cmd/server

build:
	go build -o $(BINARY_NAME) $(CMD_PATH)

run: build
	./$(BINARY_NAME)

test:
	go test -v -race ./...

clean:
	rm -f $(BINARY_NAME)
	go clean

docker-build:
	docker build -t $(BINARY_NAME) .

docker-run:
	docker run --rm -p 8080:8080 -e STORAGE_TYPE=memory $(BINARY_NAME)

migrate:
	go run $(CMD_PATH) -migrate

help:
	@echo "Available targets:"
	@echo "  build         Build the binary"
	@echo "  run           Run the service"
	@echo "  test          Run tests"
	@echo "  clean         Remove binary"
	@echo "  docker-build  Build Docker image"
	@echo "  docker-run    Run Docker container with in-memory storage"
	@echo "  migrate       Run database migrations"