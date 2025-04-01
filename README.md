# docker runtime tinkering

With great help from https://github.com/antoineco/containerd-shim-sample

# instructions

...

```
GOOS=linux GOARCH=arm64 go build -C ./shim/ . && ./scripts/macos_docker_desktop_hyperv_login.sh -nkvf ./shim/shim:/foo/bar/containerd-shim-foobar-v1
GOOS=linux GOARCH=arm64 go build -C ./brainfuck/ . && ./scripts/macos_docker_desktop_hyperv_login.sh -nkvf ./brainfuck/brainfuck:/bf/brainfuck
./scripts/macos_docker_desktop_hyperv_login.sh -nkvf ./brainfuck/hello.bf:/bf/hello.bf
./scripts/macos_docker_desktop_hyperv_login.sh -nkvf ./scripts/start-stopped.sh:/bf/start-stopped.sh
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



## bf

- https://gist.github.com/roachhd/dce54bec8ba55fb17d3a
- https://esolangs.org/wiki/Brainfuck
- https://github.com/lestrozi/pipevm/