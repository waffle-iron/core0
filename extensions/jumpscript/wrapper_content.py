import utils
import os
import sys

from multiprocessing import connection


def runner(data):
    assert 'SOCKET' in os.environ, 'SOCKET env var is not set'
    assert len(sys.argv) == 2, 'Invalid number of command line arguments'

    (path,) = sys.argv[1:]

    socket = os.environ['SOCKET']
    con = connection.Client(socket)
    exec_data = {
        'data': data['args'],  # script args
        'content': data.get('content', None),
        'path': path
    }

    con.send(exec_data)

    exception = None

    while True:
        msg = con.recv()
        if isinstance(msg, StopIteration):
            break
        elif isinstance(msg, BaseException):
            exception = msg
        else:
            sys.stdout.write(msg)
            sys.stdout.flush()

    if exception is not None:
        raise exception

    con.close()

utils.dryrun(runner)
