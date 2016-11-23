
[![Build Status](https://travis-ci.org/g8os/core.svg?branch=master)](https://travis-ci.org/g8os/core)

# Core 

Systemd replacement for G8OS

## Sample setup
The following steps will create a docker container that have core0 as the init process. When running,
u can send commands to core0 using the pyclient

First we need to prepare the base docker image to host core0
```dockerfile
FROM ubuntu:16.04
RUN apt-get update && \
    apt-get install -y wget && \
    apt-get install -y redis-server

RUN wget -O /tmp/ipfs.tgz https://dist.ipfs.io/go-ipfs/v0.4.4/go-ipfs_v0.4.4_linux-amd64.tar.gz && \
    cd /tmp && tar -xf /tmp/ipfs.tgz && cp go-ipfs/ipfs /bin
```

Make sure that you build both core0 and coreX as following
```bash
go get github.com/g8os/core0
go get github.com/g8os/coreX
```

The do 
```
cd $GOPATH/src/github.com/g8os/core0
# then start the docker container
docker run --privileged -d \
    --name core0 \
    -v `pwd`/core0:/usr/sbin/core0 \
    -v `pwd`/../coreX/coreX:/usr/sbin/coreX \
    -v `pwd`/g8os.dev.toml:/root/core.toml \
    -v `pwd`/conf:/root/conf \
    corebase \
    core0 -c /root/core.toml
```

> Note: You might ask why we do this instead of copying those files directly to the image
> the point is, now it's very easy for development, each time u rebuild the binary or change the config
> u can just do `docker restart core0` without rebuilding the whole image.

To follow the container logs do
```bash
docker logs -f core0
```

## Using the client
Before using the client make sure the `./pyclient` is in your *PYTHONPATH*

```python
import client

cl = client.Client(host='ip of docker container running core0')

#validate that core0 is reachable
print(cl.ping())

#then u can do stuff like
print(
    cl.system('ps -eF').get()
)

print(
    cl.system('ip a').get()
)

#client exposes more tools for disk, bridges, and container mgmt 
print(
    cl.disk.list()
)
```