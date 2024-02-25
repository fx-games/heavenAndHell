#!/bin/zsh

compileLinuxAMD64() {
  docker container run --rm --entrypoint='' \
      --arch=amd64 \
      -v /Users/felix/Projects/rpg:/usr/src/app \
      -w /usr/src/app \
      ghcr.io/memmaker/lfgo-amd64:latest /bin/sh -c "CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -tags ebitensinglethread -o ./linux_amd64_la -trimpath -ldflags '-s -w' ." && \
  mv ./linux_amd64_la ./builds/linux/LuxAeterna
}

compileLinuxAMD64