> Please note that this doc is out of date and might not match the current implementation

Agent Specs
===========

Defs
----

### AgentController (AC)
-   Golang written webservice who works 100% against redis (ledis) as backend

### Agent (A)
-   Installed on windows or linux node
-   Talks to AC only using HTTPS/REST (longpolling for incoming messages)

### Logger (L)
-   Coroutine in agent which is responsible for processing all messages coming in.
-   Send message to L using a Queue in the golang agent
-   L will parse the raw message ( is textual coming from e.g. subprocess)
-   L will see what to do with message
    -   Log in sqlite
    -   Batch & send to AC

### ProcessManager (PM)
-   Coroutine in agent which manages & launches all subprocesses
-   A subprocess can be a lua or python script or it can be a daemon alike subprocess (cmdline) or an internal method.
-   Process manager monitors the behavior of the subprocess & will kill/restart if required.

Agent process manager
---------------------
-   Each jumpscript or long running daemon gets launched managed by the agent in a subprocess.
-   Each of the processes is monitored per 30 sec
    -   for cpu usage, mem usage, network?, …
    -   also for the subprocesses, just aggregate them all
-   Send as statsd messages to logger (using queues internal to Golang agent)

Stats
-----
-   send all as statsd format on level 10 (see below)
-   aggregated in memory by embedded statsd aggregator
-   statsd aggregated values are send to central agentcontroller batched over rest
-   Aggregation is done once per X sec (see config) and aggregated values are sent to AC.

Logging Messages
----------------
-   All messages are send to stout or stderr, nothing goes to logserver, …
-   Process manager captures stderr & stdout & will store in a local sqlite
-   Levels are marked as
    -   $level:: at beginning of line (2x ‘:’)
    -   exception 1 & 2: they are just stdout & stderr
-   messages are multiline if
    -   $level::: has 3 x ‘:’
    -   end of multiline when ::: found at beginning of line
-   levels
    -   1: stdout
    -   2: stderr
    -   3: message for endusers / public message
    -   4: message for operator / internal message
    -   5: log msg (unstructured = level5, cat=unknown)
    -   6: log msg structured
    -   7: warning message
    -   8: ops error
    -   9: critical error
    -   10: statsd message(s)
    -   20: result message, json
    -   21: result message, yaml
    -   22: result message, toml
    -   23: result message, hrd
    -   30: job, json (full result of a job)

-   sqlite store
    -   each message gets unique id
    -   all in 1 table
    -   1 db cannot grow beyond 100 MB
    -   if 100 MB then start new DB and rename old one as $currentepoch.db
        -   new db name: current.db

### Log Message Format
-   fields
    -   jobdomain : each running process in agent is for specific domain & cmdname
    -   jobname
    -   epoch : time when log message came
    -   level
    -   id: create unique incremental id (is done per agent, restart after 4 bytes)
    -   data: is the textual representation of what came from stdout
-   stored in sqlite
    -   most of logging info will not be send to AC e.g. stdout, logs, … but can be queried by AC when required
-   when forwarded to AC do in list of dicts & send as json

identification
--------------
-   each agent has unique id
-   each agent is part of unique grid id (gid)

agent config
------------

[Agent Configuration](https://github.com/Jumpscale/jsagent/wiki/agent-configuration)

Processes
---------
-   each process is identified by
    -   domain
    -   name
-   when more than 1 process with same name & domain then the stats are aggregated for both of them before sending to AC
-   this makes that at AC name we see each process & subprocesses identified by a unique domain/name
-   the domain/name can refer to a long running jumpscript or a jumpscript which runned once
-   this aggregation on this level will allow us to aggregate on rack, datacenter, region level

Cmds
----
Agentcontroller sends messages to Agent.
Agent connects to AC using long polling over rest
### Cmd Message format (result of executing a cmd is a job)
-   JSON
-   Dict
    -   id:$id unique id given by requester (is id for the job)
    -   gid:grid id is unique
    -   nid:node id, only 1 agent per node, is unique per grid
    -   cmd:$name of command (for list of cmds see below)
    -   args:$dict with arguments relevant for cmd
    -   data:$value depending cmd means something different
-   return is again a json dict
    -   id:$id unique id given by requester
    -   gid:grid id is unique
    -   nid:node id, only 1 agent per node, is unique per grid
    -   cmd:$name of command
    -   args:$dict with arguments for cmd
    -   data:return data
    -   state: OK, ERROR, TIMEOUT, KILLED
    -   time: time it took in msec
    -   starttime: when job started on agent (msec)

