all: build docker-build run

build:
	mkdir -p ./tmp
	CGO_ENABLED=0 go build -o ./tmp/builder .

docker-build: build
	docker build --build-arg BUILD_DIR=./tmp -t go-builder -f Dockerfile.golang .
	docker build --build-arg BUILD_DIR=./tmp -t cpp-builder -f Dockerfile.gcc .

run: docker-build
	./tmp/builder --listen :8081