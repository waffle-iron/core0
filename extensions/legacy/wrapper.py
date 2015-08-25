import utils
import os
import socket
import errno
import executor


EXEC_SOCKET = '/tmp/legacy.sock'


def runner(data):
    jspath = os.environ.get('JUMPSCRIPTS_HOME')
    assert jspath is not None, 'Missconfigured, no jspath define'

    sock = None
    daemon_running = False
    # we use the lock to avoid having multiple processes trying to
    # start the executor at the same time.

    with utils.Lock('/tmp/legacy.lock'):
        trial = 0
        while True:
            try:
                trial += 1
                sock = socket.socket(socket.AF_UNIX)
                sock.connect(EXEC_SOCKET)
                break
            except Exception, e:
                if trial >= 3:
                    raise Exception('Failed to establish connection with the executor')

                if e.errno in (errno.ENOENT, errno.ECONNREFUSED):
                    # start the executor deamon.
                    if not daemon_running:
                        executor.daemon(EXEC_SOCKET)
                        daemon_running = True
                else:
                    raise

    con = executor.Connection(sock)
    con.send(data)

    response = con.receive()

    if isinstance(response, BaseException):
        raise response

    con.close()
    return response


utils.run(runner)
