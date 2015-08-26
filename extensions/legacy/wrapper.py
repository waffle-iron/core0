import utils
import os
import errno
import executor
import time
from multiprocessing import connection


EXEC_SOCKET = '/tmp/legacy.sock'


class NoSocketError(OSError):
    errno = errno.ENOENT


def runner(data):
    jspath = os.environ.get('JUMPSCRIPTS_HOME')
    assert jspath is not None, 'Missconfigured, no jspath define'

    con = None
    daemon_running = False
    # we use the lock to avoid having multiple processes trying to
    # start the executor at the same time.
    with utils.Lock('/tmp/legacy.lock'):
        trial = 0
        while True:
            try:
                trial += 1
                if not os.path.exists(EXEC_SOCKET):
                    raise NoSocketError()

                con = connection.Client(EXEC_SOCKET)
                break
            except Exception, e:
                if trial >= 3:
                    raise Exception('Failed to establish connection with the executor')

                if e.errno in (errno.ENOENT, errno.ECONNREFUSED):
                    # start the executor deamon.
                    if not daemon_running:
                        executor.daemon(EXEC_SOCKET)
                        daemon_running = True
                    time.sleep(0.5)
                else:
                    raise

    con.send(data)

    response = con.recv()

    if isinstance(response, BaseException):
        raise response

    con.close()
    return response


utils.run(runner)
