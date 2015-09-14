import utils
import os
from multiprocessing import connection


def runner(data):
    assert 'SOCKET' in os.environ, 'SOCKET env var is not set'

    socket = os.environ['SOCKET']
    con = connection.Client(socket)
    con.send(data)

    response = con.recv()

    if isinstance(response, BaseException):
        raise response

    con.close()
    return response


utils.run(runner)
