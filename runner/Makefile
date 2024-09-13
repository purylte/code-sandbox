all: build docker-build run

build:
	mkdir -p ./tmp
	go build -o ./tmp/runner .

docker-build: build
	docker build --build-arg BUILD_DIR=./tmp -t runner -f Dockerfile.gvisor .

run: docker-build
	./tmp/runner --listen :8082