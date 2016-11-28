Core0 supports logging modules to be implemented, to store the command logs
to different log aggregation stores.

## Logger interface
The Logger interface has a single method Log() that takes both the cmd
and the logged message object.

Currently we only have a valid implementation (ConsoleLogger) which
prints the message (if the log level is high enough) to the console.

## Requirement
We need to do the following
- Move the Logger interface and implementation to `core.base` since this will be used by both core0 and coreX
- Implement a redis logger that pushes the messages to a redis queue. The object pushed into redis must have a good structure for processing later
- Logger must make sure to not put redis out of memory by making sure that the redis queue doesn't grow over than (100,000 record - configurable)
- If redis queue is growing over the limit the older records are dropped (check redis commands, it already has built in support for this)

### CoreX logs forwarding to Core0
On a single host, there are 2 redis instances working. A one that is managed to control core0 and is accessible from the network.
This redis is where we need to eventually keep all the logs, so they can be pulled later by external apps.
Another instance is running internally and is not accessible from outside, and is used to manage coreX. This is the only instance
that the coreX can access.

So coreX needs to make sure to push all it's logs to the local redis instance, and core0 (which has access to both) need to make sure
to start a routing that pulls logs from the local redis to the global redis instance.


```
coreX(1..) -> pushes logs to local redis (queue: 'core.logs')
core0 -> pushes it's own logs to public redis (queue: 'core.logs') 
core0 -> runs a routine that copy logs from local(core.logs) to public(core.logs)
``` 

- We need to end up with all the logs pushed to `public(core.logs)` queue.

### Suggested logs structure
```jsavscript
{
    core: 0, //0, 1, 2... depending on the source core
    command: Command //as provided to the Logger.Log method
    message: Message //as provided to the Logger.Log method
}
```