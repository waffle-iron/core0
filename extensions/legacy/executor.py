import os
import threading
import imp
import sys
import logging
import resource
import signal

from multiprocessing import Process, connection

LOG_FORMAT = '%(asctime)-15s [%(process)d] %(levelname)s: %(message)s'


class WrapperThread(Process):
    def __init__(self, con):
        self.con = con
        super(WrapperThread, self).__init__()

    def run(self):
        try:
            data = self.con.recv()
            jspath = os.environ.get('JUMPSCRIPTS_HOME')
            path = os.path.join(jspath, data['domain'], '%s.py' % data['name'])
            logging.info('Executing jumpscript: %s' % data)

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

    os.closerange(0, resource.RLIMIT_NOFILE)
    os.setsid()
    os.umask(0)

    try:
        pid = os.fork()
        if pid > 0:
            # parent
            sys.exit(0)
    except OSError, e:
        logging.error(e)
        sys.exit(1)

    logging.basicConfig(filename='/var/log/legacy.log',
                        format=LOG_FORMAT,
                        level=logging.INFO)
    try:
        os.unlink(unix_sock_path)
    except:
        pass

    logging.info('Starting daemon')
    try:
        listner = connection.Listener(address=unix_sock_path)

    except Exception, e:
        logging.error('Could not start listening: %s' % e)

    def exit(n, f):
        logging.info('Stopping daemon')
        listner.close()
        sys.exit(1)

    for s in (signal.SIGTERM, signal.SIGHUP, signal.SIGQUIT):
        signal.signal(s, exit)

    # importing jumpscale
    from JumpScale import j # NOQA

    while True:
        try:
            c = listner.accept()
            p = WrapperThread(c)
            p.start()
            # We need this (p.join) to avoid having zombi processes
            # But we wait in a thread for the subporcess to exit to avoid
            # blokcing the main loop.
            threading.Thread(target=p.join).start()

        except Exception, e:
            logging.error(e)
