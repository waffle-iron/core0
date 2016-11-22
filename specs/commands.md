> This document is a continues WIP

#Core0/CoreX communication protocol

## Core0
First process to start on bare metal. It works as a simple process manager.
When starts it first configure the networking then. It also starts a local redis instance to dispatch commands to CoreX Cores.

## Command structure

```javascript
{
	"id": "command-id",
	"command": "command-name",
	"arguments": {}, //command arguments depends on the command itself
	"queue": "optional-queue",
	"stats_interval": 0, //optional stats gathering interval (falls to default if not set)
	"max_time": 0, //Max run time of the command, if exceeded command will be killed
	"max_restart": 0, //Max number of retries to start the command if failed before giving up
	"recurring_period": 0, //If provided command is considered recurring
	"log_levels": [int] //Log levels to store locally and not discard.
}
```

The `Core0` Core understands a very specific set of management commands:

- Basic
 - core.ping
 - core.system
 - core.kill
 - core.killall
 - core.state
 - core.reboot
- Info
 - info.cpu
 - info.disk
 - info.mem
 - info.nic
 - info.os
- CoreX
 - corex.create
 - corex.list
 - corex.dispatch
 - corex.terminate
- Bridge
 - bridge.create
 - bridge.list
 - bridge.delete

## Commands arguments
### core.ping
Doesn't take any arguments. returns a "pong". Main use case is to check the the core is responding.

### core.system
Arguments:
```javascript
{
	"name": "executable",
	"dir": "pwd",
	"args": ["command", "arguements"]
	"env": {"ENV1": "VALUE1", "ENV2": "VALUE2"},
	"stdin": "data to pass to executable over stdin"
}
```
Executes an arbitrary command

### core.kill
Arguments:
```javascript
{
    "id": "process-id-to-kill"
}
```
Kills a certain process giving the process ID. The process/command id is the id of the command used to start this process
in the first place.

### core.killall
Takes no arguments
Kills ALL processes on the system. (only the ones that where started by core0 itself) and still running by the time of calling this command

### core.state
Takes no arguments.
Returns aggregated state of all processes plus the consumption of core0 itself (cpu, memory, etc...)

### core.reboot
Takes no arguments.
Immediately reboot the machine.

### info.cpu
Takes no arguments.
Returns information about the host CPU types, speed and capabilities

### info.disk
Takes no arguments.
Returns information about the host attached disks

### info.mem
Takes no arguments.
Returns information about the host memory.

### info.nic
Takes no arguments.
Returns information about the host attached nics, and IPs

### info.os
Takes no arguments.
Returns information about the host OS.

### corex.create
Arguments:
```
{
    "plist": "http://url/to/plist",
    "mount": {
        "/host/directory": "/container/directory"
    },
    "network": {
        "zerotier": "zerotier network id", //options
        "bridge": [], //list of bridges names to connect to
    }
    //TODO:
    "port": {
        host_port: container_port,
    }
}
```

### corex.list
Takes no arguments.
List all available `live` Cores on a host.

### corex.terminate
Arguemnts:
```javascript
{
    "container": container_id,
}
```
Destroys a Core and stops the Core processes. It takes a mandatory core ID.

### corex.dispatch
Arguments:
```javascript
{
     "container": core_id,
     "command": {
         //the full command payload
     }
}
```

### bridge.create
Arguments:
```javascript
{
    "name": "bridge-name", //required
    "hwaddr": "MAC address" //optional
}
```
Creates a new bridge

### bridge.list
takes no argumetns
List all available bridges

### bridge.delete
Arguments:
```javascript
{
    "name": "bridge-name", //required
}
```
Delete the given bridge name