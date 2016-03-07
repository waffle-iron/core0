# Core

Systemd replacement for G8OS

## Sample setup
- Build and copy `core` to machine /sbin/
- `mkdir -p /etc/g8os/g8os.d/`
- Write file `/etc/g8os/g8os.toml` with the following contents
```toml
#leave /tmp and /opt/jumpscale8 in this file, gets replaced during installation

[main]
max_jobs = 100
message_ID_file = "/var/run/core.mid"
include = "/etc/g8os/g8os.d"
network = "/etc/g8os/network.toml"

[channel]
cmds = [] # empty for long polling from all defined controllers, or specif controllers keys

[extension]
    [extension.bash]
    binary = "bash"
    args = ['-c', 'T=`mktemp` && cat > $T && chmod +x $T && bash -c $T; EXIT=$?; rm -rf $T; exit $EXIT']

[logging]
    [logging.console]
    type = "console"
    levels = [1, 2]

[stats]
interval = 60000 # milliseconds (1 min)
[stats.ac]
    enabled = false
    controllers = [] # empty for all controllers, or controllers keys
[stats.redis]
    enabled = false
    flush_interval = 100 # millisecond
    address = "localhost:6379"

[hubble]
controllers = [] # accept forwarding commands and connections from all controllers. Or specific controllers by name
```
- Write file `/etc/g8os/network.toml` with content
```
[network]
auto = true
```
- cp file `conf/getty.toml`, `conf/sshd.toml` and `conf/modprobe.toml` from the source tree to `/etc/g8os/g8os.d`
- cp the `init` file from the source tree to `/sbin/init` (make sure to backup your original `init` file so you
can go back to `systemd`

Reboot machine