.PHONY: run install uninstall

all: run

./shim/main: ./shim/main.go
	go build -o ./shim/main ./shim/main.go

./shim/main-arm64: ./shim/main.go
	GOOS=linux GOARCH=arm64 go build -o ./shim/main-arm64 ./shim/main.go

run: ./shim/main
	./shim/main


install: ./shim/main-arm64
	chmod +x ./scripts/macos_docker_desktop_hyperv_login.sh
	./scripts/macos_docker_desktop_hyperv_login.sh -nvk -f ./shim/main-arm64:foo/bar/shim

# for now we hack the shim uninstall by tunnelling into the hypervisor without the -k flag so that the
# shim is deleted after exit
uninstall:
	chmod +x ./scripts/macos_docker_desktop_hyperv_login.sh
	./scripts/macos_docker_desktop_hyperv_login.sh -nv -f ./shim/main-arm64:foo/bar/shim

shimmed-alpine:
	docker run --runtime foo -it --rm alpine:latest

clean:
	rm -f ./shim/main
	rm -f ./shim/main-arm64
	rm -f ./shim/shim.log
	