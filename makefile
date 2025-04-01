.PHONY: run install uninstall docker clean build hello

all: hello

SRC=./cmd/containerd-shim-brainfuck-v1.go
BIN_NAME=containerd-shim-brainfuck-v1

${BIN_NAME}-native: ${SRC}
	go build  -o ${BIN_NAME}-native ${SRC}

${BIN_NAME}-arm64: ${SRC}
	GOOS=linux GOARCH=arm64 go build -o ${BIN_NAME}-arm64 ${SRC}

build: ${BIN_NAME}-native ${BIN_NAME}-arm64

hello: ${BIN_NAME}-native
	./${BIN_NAME}-native brainfuck -file ./bf/programs/hello.bf

install: ${BIN_NAME}-arm64
	chmod +x ./scripts/macos_docker_desktop_hyperv_login.sh
	./scripts/macos_docker_desktop_hyperv_login.sh -nkvf ./${BIN_NAME}-arm64:/usr/bin/containerd-shim-brainfuck-v1

# for now we hack the shim uninstall by tunnelling into the hypervisor without the -k flag so that the
# shim is deleted after exit
uninstall: ${BIN_NAME}-arm64
	chmod +x ./scripts/macos_docker_desktop_hyperv_login.sh
	./scripts/macos_docker_desktop_hyperv_login.sh -nvf ./${BIN_NAME}-arm64:/usr/bin/containerd-shim-brainfuck-v1

docker: Dockerfile
	docker build --file=Dockerfile -t bf .
	docker run --rm --runtime brainfuck --network none -t bf:latest

clean: uninstall
	docker rmi -f bf:latest
	rm -f ${BIN_NAME}-native
	rm -f ${BIN_NAME}-arm64
	