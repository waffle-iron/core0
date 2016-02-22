# Agent8 #


### Functions of a Agent8
- is a process manager
- a remote command executor that gets it's jobs and tasks by polling from AC (Agent Controller 2).
- tcp portforwarder
- statistics aggregator & forwarder

The Agent will also monitor the jobs, updating the AC with `stats` and `logs`. All according to specs. 

# Architecture

![](https://docs.google.com/drawings/d/1qsOzbv2XbwChgsLVV8qCydmH0ki9QLkaB336kt7D1Cg/pub?w=960&h=720)

For more information checkout the [docs](https://gig.gitbooks.io/jumpscale8/content/MultiNode/AgentController2/AgentController2.html#).
