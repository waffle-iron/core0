import utils
import os
import sys

from multiprocessing import connection


MODE_MODERN = 'modern'
MODE_LEGACY = 'legacy'


def runner(data):
    assert 'SOCKET' in os.environ, 'SOCKET env var is not set'
    assert len(sys.argv) == 4, 'Invalid number of command line arguments'

    mode, domain, name = sys.argv[1:]
    assert mode in (MODE_MODERN, MODE_LEGACY), 'Only support execution modes are modren and legacy'

    socket = os.environ['SOCKET']
    con = connection.Client(socket)
    exec_data = {
        'data': data,
        'legacy': mode == MODE_LEGACY,
        'domain': domain,
        'name': name
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
