# docker runtime tinkering

With great help from https://github.com/antoineco/containerd-shim-sample

# instructions

...

Buld shim and binary:

```
GOOS=linux GOARCH=arm64 go build ./cmd/containerd-shim-brainfuck-v1.go && ./scripts/macos_docker_desktop_hyperv_login.sh -nkvf ./containerd-shim-brainfuck-v1:/usr/bin/containerd-shim-brainfuck-v1
GOOS=linux GOARCH=arm64 go build ./bf/cmd/brainfuck.go && ./scripts/macos_docker_desktop_hyperv_login.sh -nkvf ./brainfuck:/usr/bin/brainfuck
```

Build dockerfile and run:

```
docker build --file=Dockerfile -t hello .
docker run --rm --runtime brainfuck --network none -t hello:latest
```

# links

- https://stackoverflow.com/questions/76383059/what-is-the-architecture-overview-of-docker-desktop-on-mac
- https://docs.docker.com/desktop/troubleshoot-and-support/faqs/macfaqs/
- https://stackoverflow.com/questions/39739560/how-to-access-the-vm-created-by-dockers-hyperkit
- https://docs.docker.com/reference/cli/docker/container/run/
- https://github.com/antoineco/containerd-shim-sample



## bf

- https://gist.github.com/roachhd/dce54bec8ba55fb17d3a
- https://esolangs.org/wiki/Brainfuck
- https://github.com/lestrozi/pipevm/