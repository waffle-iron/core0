import utils
import os
import sys

from multiprocessing import connection


def runner(data):
    assert 'SOCKET' in os.environ, 'SOCKET env var is not set'
    assert 'AGENT_CONTROLLER_URL' in os.environ, 'Missing AGENT_CONTROLLER_URL'
    assert 'AGENT_GID' in os.environ, 'Missing AGENT_GID'
    assert 'AGENT_NID' in os.environ, 'Missing AGENT_NID'

    socket = os.environ['SOCKET']
    con = connection.Client(socket)

    # Hash will get processed by the daemon as follows
    # It will contact the agentcontroller to retrieve the cached script
    # and execute it normally.
    # we also collect the controller variables from env vars.
    controller = {
        'gid': os.environ['AGENT_GID'],
        'nid': os.environ['AGENT_NID'],
        'url': os.environ['AGENT_CONTROLLER_URL'],
        'name': os.environ['AGENT_CONTROLLER_NAME'],
        'ca': os.environ.get('AGENT_CONTROLLER_CA', None),
        'client_cert': os.environ.get('AGENT_CONTROLLER_CLIENT_CERT', None),
        'client_cert_key': os.environ.get('AGENT_CONTROLLER_CLIENT_CERT_KEY', None)
    }

    exec_data = {
        'data': data['args'],  # script args
        'hash': data.get('hash', None),
        'controller': controller
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
