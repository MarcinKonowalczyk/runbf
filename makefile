myshim:
	GOOS=linux GOARCH=arm64 go build -o myshim ./shim/main.go

install:
	chmod +x ./scripts/macos_docker_desktop_hyperv_login.sh
	./scripts/macos_docker_desktop_hyperv_login.sh -nvk -f myshim:foo/bar/shim

# for now we hack the shim uninstall by tunelling into the hypervisor without the -k flag so that the
# shim is deleted after exit
uninstall:
	chmod +x ./scripts/macos_docker_desktop_hyperv_login.sh
	./scripts/macos_docker_desktop_hyperv_login.sh -nv -f myshim:foo/bar/shim