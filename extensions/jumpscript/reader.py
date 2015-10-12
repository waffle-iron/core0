import sys


def readResponseToEnd(con):
    exception = None
    streams = {
        1: sys.stdout,
        2: sys.stderr
    }

    while True:
        streamNum, msg = con.recv()
        if isinstance(msg, StopIteration):
            break
        elif isinstance(msg, BaseException):
            exception = msg
        else:
            if streamNum in streams:
                stream = streams[streamNum]
                stream.write(msg)
                stream.flush()

    if exception is not None:
        # this means an error in the tasklet. and must exit with an error exit code.
        return 1
    return 0
