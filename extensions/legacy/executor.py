import os
import threading
import cPickle as pickle
import socket
import errno
import imp
import sys
import logging
import struct
import resource


class Connection(object):
    HEADER_FMT = '@i'
    HEADER_SIZE = struct.calcsize(HEADER_FMT)

    def __init__(self, sock):
        self.sock = sock

    def _recv_size(self, size):
        data = ''
        while len(data) < size:
            data += self.sock.recv(size - len(data))

        return data

    def receive(self):
        header = self._recv_size(self.HEADER_SIZE)
        size = struct.unpack(self.HEADER_FMT, header)[0]

        raw_data = self._recv_size(size)
        return pickle.loads(raw_data)

    def send(self, message):
        data = pickle.dumps(message)
        header = struct.pack(self.HEADER_FMT, len(data))
        self.sock.sendall(header)
        self.sock.sendall(data)

    def close(self):
        self.sock.close()


class WrapperThread(threading.Thread):
    def __init__(self, con):
        self.con = con
        super(WrapperThread, self).__init__()

    def run(self):
        try:
            data = self.con.receive()
            jspath = os.environ.get('JUMPSCRIPTS_HOME')
            path = os.path.join(jspath, data['domain'], '%s.py' % data['name'])
            logging.info('Executing jumpscript %s' % path)

            module = imp.load_source(path, path)
            result = module.action(**data['args'])

            self.con.send(result)
        except Exception, e:
            logging.error(e)
            self.con.send(e)
        finally:
            self.con.close()


def daemon(unix_sock_path):
    # socket is ready, now do the daemon forking
    pid = os.fork()
    if pid > 0:
        # parent.
        os.waitpid(pid, os.P_WAIT)
        return

    os.setsid()
    os.umask(0)

    os.closerange(0, resource.RLIMIT_NOFILE)

    sock = socket.socket(socket.AF_UNIX)
    try:
        os.unlink(unix_sock_path)
    except OSError, e:
        if e.errno != errno.ENOENT:
            raise

    sock.bind(unix_sock_path)
    sock.listen(10)

    logging.basicConfig(filename='/var/log/legacy.log', level=logging.INFO)
    # double fork to avoid zombies
    logging.info('Starting executor daemon')

    try:
        pid = os.fork()
        if pid > 0:
            # parent
            sys.exit(0)
    except OSError, e:
        logging.error(e)
        sys.exit(1)

    # importing jumpscale
    from JumpScale import j # NOQA

    while True:
        s = sock.accept()[0]
        WrapperThread(Connection(s)).start()
