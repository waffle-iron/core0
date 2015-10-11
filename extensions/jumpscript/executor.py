import os
import threading
import imp
import sys
import logging
import signal
import utils
import json
import requests
from multiprocessing import Process, connection
import traceback

# importing jumpscale
from JumpScale import j # NOQA

import logger


LOG_FORMAT = '%(asctime)-15s [%(process)d] %(levelname)s: %(message)s'
SCRIPTS_CACHE_DIR = '/tmp/jscache/'
SCRIPTS_URL_PATH = '{gid}/{nid}/script'


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

    def get_script_path(self, js_hash, controller):
        script_name = '%s.js' % js_hash
        path = os.path.join(SCRIPTS_CACHE_DIR, script_name)
        if os.path.exists(path):
            return path

        # get the file.
        url_path = SCRIPTS_URL_PATH.format(gid=controller['gid'], nid=controller['nid'])
        url = '{url}/{path}'.format(url=controller['url'].rstrip('/'), path=url_path)

        args = {
            'verify': False,  # don't care about server certificate.
        }

        if controller['client_cert'] is not None:
            args['cert'] = (controller['client_cert'], controller['client_cert_key'])

        response = requests.get(url, params={'hash': js_hash}, **args)
        if not response.ok:
            raise Exception('Failed to retrieve script from controller %s' % response.reason)

        j.system.fs.writeFile(path, response.content)
        return path

    def run_with_content(self, data):
        args = data['data']
        js_hash = data['hash']
        controller = data['controller']

        path = self.get_script_path(js_hash, controller)

        return self.run_path(path, args)

    def run(self):
        try:
            data = self.con.recv()

            j.logger = logger.LogHandler(self.con)
            if 'domain' in data:
                result = self.run_with_domain_name(data)
            elif 'hash' in data:
                result = self.run_with_content(data)

            j.logger.log(json.dumps(result), j.logger.RESULT_JSON)

        except Exception, e:
            error = traceback.format_exc()
            logging.error(error)
            eco = j.errorconditionhandler.parsePythonErrorObject(e)
            j.logger.log(eco.toJson(), j.logger.LOG_CRITICAL)
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

    # make sure we create cache folder.
    j.system.fs.createDir(SCRIPTS_CACHE_DIR)

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
