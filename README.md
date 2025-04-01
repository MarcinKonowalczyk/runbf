# runbf

[Containerd](https://github.com/containerd/containerd) [v2 runtime shim](https://github.com/containerd/containerd/blob/main/core/runtime/v2/README.md) for [brainfuck](https://esolangs.org/wiki/Brainfuck).

This works analogously to [WASI runtimes](https://github.com/containerd/runwasi). The container consists of a single entrypoint:

```dockerfile
FROM scratch
COPY bf/programs/factors.bf /
CMD ["/factors.bf"]
```

which can then (after installing the shim, see below) be run with:

`docker run --rm --runtime brainfuck --network none -t bf:latest`

# instructions

I've developed this on macos so step 1) was to hack into the vm in which 'Docker Desktop' runs its containers. TLDR; this is done with the `./scripts/macos_docker_desktop_hyperv_login.sh` script.

In general the shim ought to work on any platform though. Just follow the installation instructions for containerd shims for your os / adapt the instructions below.

**1)** Build shim (my vm is `Linux docker-desktop 6.10.14-linuxkit aarch64 GNU/Linux`)

```sh
GOOS=linux GOARCH=arm64 go build ./cmd/containerd-shim-brainfuck-v1.go
```

**2)** Place it somewhere containerd can find it (here is where i need to hack into the vm): 

```sh
./scripts/macos_docker_desktop_hyperv_login.sh -nkvf ./containerd-shim-brainfuck-v1:/usr/bin/containerd-shim-brainfuck-v1
```

*NOTE:* If, like me, you're adding the runtime to a volative vm, you will need to redo this if tou restart docker-desktop.

**3)** Add the runtime to docker daemon config (you will need to restart it afterwards)

```sh
jq ".runtimes += {\"brainfuck\": {\"runtimeType\": \"/usr/bin/containerd-shim-brainfuck-v1\"}}" ~/.docker/daemon.json >/tmp/daemon.json
cat /tmp/daemon.json >~/.docker/daemon.json
```
**4)** Build dockerfile and run:

```sh
docker build --file=Dockerfile -t bf .
docker run --rm --runtime brainfuck --network none -t bf:latest
``` 

# dev

You can read the containerd logs with:

```sh
tail -f ~/Library/Containers/com.docker.docker/Data/log/vm/containerd.log
```

# links

## docker

- https://github.com/antoineco/containerd-shim-sample
- https://stackoverflow.com/questions/76383059/what-is-the-architecture-overview-of-docker-desktop-on-mac
- https://docs.docker.com/desktop/troubleshoot-and-support/faqs/macfaqs/
- https://stackoverflow.com/questions/39739560/how-to-access-the-vm-created-by-dockers-hyperkit
- https://docs.docker.com/reference/cli/docker/container/run/
- https://github.com/antoineco/containerd-shim-sample
- https://docs.docker.com/engine/daemon/logs/

## bf

- https://gist.github.com/roachhd/dce54bec8ba55fb17d3a
- https://esolangs.org/wiki/Brainfuck
- https://github.com/lestrozi/pipevm/
- https://esolangs.org/wiki/Mastermind