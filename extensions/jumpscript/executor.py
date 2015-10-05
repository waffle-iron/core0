import os
import threading
import imp
import sys
import logging
import signal
import utils
import json
from multiprocessing import Process, connection
import traceback

# importing jumpscale
from JumpScale import j # NOQA

import logger


LOG_FORMAT = '%(asctime)-15s [%(process)d] %(levelname)s: %(message)s'


class WrapperThread(Process):
    def __init__(self, con):
        self.con = con
        super(WrapperThread, self).__init__()

    def run_path(self, path, args):
        logging.info('Executing jumpscript: %s' % path)
        module = imp.load_source(path, path)
        result = module.action(**args)

        return result

    def run_with_domain_name(self, data):
        jspath = os.environ.get('JUMPSCRIPTS_HOME')
        path = os.path.join(jspath, data['domain'], '%s.py' % data['name'])

        return self.run_path(path, data['data'])

    def run_with_content(self, data):
        args = data['data']
        content = data['content']
        path = data['path']

        temp = None

        if content:
            temp = j.system.fs.getTempFileName(prefix='jumpscript.')
            j.system.fs.writeFile(temp, content)
            path = temp

        try:
            return self.run_path(path, args)
        finally:
            if temp:
                j.system.fs.remove(temp)
                j.system.fs.remove('%sc' % temp)  # remove the compiled version

    def run(self):
        try:
            data = self.con.recv()

            j.logger = logger.LogHandler(self.con)
            if 'domain' in data:
                result = self.run_with_domain_name(data)
            elif 'content' in data:
                result = self.run_with_content(data)

            j.logger.log(json.dumps(result), j.logger.RESULT_JSON)

        except Exception, e:
            error = traceback.format_exc()
            logging.error(error)
            self.con.send(e)
        finally:
            self.con.send(StopIteration())
            self.con.close()


def daemon(data):
    assert 'SOCKET' in os.environ, 'SOCKET env var is not set'
    assert 'JUMPSCRIPTS_HOME' in os.environ, 'JUMPSCRIPTS_HOME env var is not set'

    unix_sock_path = os.environ.get('SOCKET')
    try:
        os.unlink(unix_sock_path)
    except:
        pass

    logging.basicConfig(filename='/var/log/jumpscript-daemon.log',
                        format=LOG_FORMAT,
                        level=logging.INFO)

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

utils.run(daemon)
