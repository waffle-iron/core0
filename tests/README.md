# How to run integration tests under here
To run the integration tests, you need to `go get github.com/Jumpscale/agentcontroller8` because the tests starts an 
instance of the `agentcontroller8` automatically.

Also you need to have a working redis instance (with no passwords) running on the local machine and listining on the standard
port `6379`

Then to run the tests:
```bash
go test -v
```
