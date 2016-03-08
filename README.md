# Core

Systemd replacement for G8OS

## Sample setup
- Build and copy `core` to machine /sbin/
- `mkdir -p /etc/g8os/g8os.d/`
- Write file `/etc/g8os/g8os.toml` with the following contents
```toml
[main]
max_jobs = 200
message_ID_file = "/var/run/core.mid"
include = "/etc/g8os/g8os.d"
network = "/etc/g8os/network.toml"

[channel]
cmds = [] # empty for long polling from all defined controllers, or specif controllers keys

#the very basic agent extensions. Also please check the toml files under
#the Main.Include folder for more extensions
[extension.syncthing]
binary = "syncthing"
cwd = "/usr/lib/g8os/extensions/"
args = ["-home", "./syncthing", "-gui-address", "127.0.0.1:28384"]

[extension.sync]
#syncthing extension
binary = "python"
cwd = "/usr/lib/g8os/extensions/sync"
args = ["{name}.py"]
    [extension.sync.env]
    PYTHONPATH = "../:/opt/jumpscale8/lib"
    SYNCTHING_URL = "http://localhost:28384"

[extension.jumpscript]
binary = "python"
cwd = "/usr/lib/g8os/extensions/jumpscript"
args = ["wrapper.py", "{domain}", "{name}"]
    [extension.jumpscript.env]
    SOCKET = "/tmp/jumpscript.sock"
    PYTHONPATH = "../"

[extension.jumpscript_content]
binary = "python"
cwd = "/usr/lib/g8os/extensions/jumpscript"
args = ["wrapper_content.py"]
    [extension.jumpscript_content.env]
    SOCKET = "/tmp/jumpscript.sock"
    PYTHONPATH = "../"

[extension.js_daemon]
binary = "python"
cwd = "/usr/lib/g8os/extensions/jumpscript"
args = ["executor.py"]
    [extension.js_daemon.env]
    SOCKET = "/tmp/jumpscript.sock"
    PYTHONPATH = "../:/opt/jumpscale8/lib"
    JUMPSCRIPTS_HOME = "/opt/jumpscale8/apps/agent8/jumpscripts/"

[extension.bash]
binary = "bash"
args = ['-c', 'T=`mktemp` && cat > $T && bash $T; EXIT=$?; rm -rf $T; exit $EXIT']

[logging]
    [logging.db]
    type = "DB"
    address = "./logs"
    levels = [2, 4, 7, 8, 9, 11]  # (all error messages + debug) empty for all

    [logging.console]
    type = "console"
    levels = [2, 4, 7, 8, 9]

[hubble]
controllers = [] # accept forwarding commands and connections from all controllers. Or specific controllers by name

```
- Write file `/etc/g8os/network.toml` with content
```
[network]
auto = true

#please leave the lo settings as is, otherwise your
#lo device won't have an IP.
[interface.lo]
protocol = "static"

[static.lo]
ip = "127.0.0.1/8"
```
- cp file `conf/basic.toml` `conf/getty.toml`, `conf/sshd.toml` and `conf/modprobe.toml` from the source tree to `/etc/g8os/g8os.d`
- cp the `init` file from the source tree to `/sbin/init` (make sure to backup your original `init` file so you
can go back to `systemd`

Reboot machine