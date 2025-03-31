# docker runtime tinkering

With great help from https://github.com/antoineco/containerd-shim-sample

# instructions

...

```
GOOS=linux GOARCH=arm64 go build -C ./shim/ . && ./scripts/macos_docker_desktop_hyperv_login.sh -nkvf ./shim/shim:/foo/bar/containerd-shim-foobar-v1
```

```
docker run --rm --runtime io.containerd.skeleton.v1 --network none -t docker.io/library/hello-world:latest hello  
```

# links

- https://stackoverflow.com/questions/76383059/what-is-the-architecture-overview-of-docker-desktop-on-mac
- https://docs.docker.com/desktop/troubleshoot-and-support/faqs/macfaqs/
- https://stackoverflow.com/questions/39739560/how-to-access-the-vm-created-by-dockers-hyperkit
- https://docs.docker.com/reference/cli/docker/container/run/
- https://github.com/antoineco/containerd-shim-sample