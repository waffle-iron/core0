import utils
import os
import sys

from multiprocessing import connection


def runner(data):
    assert 'SOCKET' in os.environ, 'SOCKET env var is not set'
    assert len(sys.argv) == 3, 'Invalid number of command line arguments'

    domain, name = sys.argv[1:]
    socket = os.environ['SOCKET']
    con = connection.Client(socket)
    exec_data = {
        'data': data,
        'domain': domain,
        'name': name
    }

    con.send(exec_data)

    response = con.recv()

    if isinstance(response, BaseException):
        raise response

    con.close()
    return response


utils.run(runner)
