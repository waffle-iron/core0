This document is about testing the core as a standalone process manager (no connection to the controller is required)

## Start the core
You can start a very simple stripped down version of the `core` by doing the following

```bash
core -gid 1 -nid 1 -c example-standalone.toml
```

If you inspected the example config file, you can see that it doesn't have most of the built in exensions. It also doesn't include
config director or network configuration. 

You still can do a lot using this stripped down version by using the `corectl` tool to manage it and start services inside 
the core. Please refer to the [corectrl](https://github.com/g8os/corectl) for more info on how to use this tool.

## Make the core connect to a controller
First of all start a `controller` by following instructions on how to start it from [controller](https://github.com/g8os/controller)

Then add the followin section to the `core` config file 

```toml
[controllers.main]
url = "http://localhost:8966"
```

The restart the core

Now you can use the jumpscale client to manage the core.
