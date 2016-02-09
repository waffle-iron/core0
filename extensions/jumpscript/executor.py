import os
import imp
import sys
import logging
import signal
import utils
import json
import requests
import time
from multiprocessing import Process, connection, Pool, reduction
import traceback
import tempfile

# importing jumpscale
from JumpScale import j # NOQA

import logger


LOG_FORMAT = '%(asctime)-15s [%(process)d] %(levelname)s: %(message)s'

SCRIPTS_CACHE_DIR = '%s/jscache/' % tempfile.gettempdir()
SCRIPTS_URL_PATH = '{gid}/{nid}/script'
SCRIPTS_DELETE_OLDER_THAN = 86400  # A day

POOL_SIZE = 10
MAX_JOBS_PER_WORKER = 100


class WrapperThread(object):
    def __init__(self, con):
        # remember this is running inside a fork now. So monkey patching only affecting this process
        self.con = con
        sys.stdout = logger.StreamHandler(con, 1)
        sys.stderr = logger.StreamHandler(con, 2)

    def run_path(self, path, args):
        logging.info('Executing jumpscript: %s' % path)
        module = imp.load_source(path, path)
        result = module.action(**args)

        return result

    def run_with_domain_name(self, data):
        jspath = os.environ.get('JUMPSCRIPTS_HOME')
        agent_name = data['controller-name']
        path = os.path.join(jspath, agent_name, data['domain'], '%s.py' % data['name'])

        if not j.sal.fs.exists(path):
            raise ValueError('Jumpscript %s/%s does not exist' % (data['domain'], data['name']))

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

        j.sal.fs.writeFile(path, response.content.decode())
        return path

    def run_with_content(self, data):
        args = data['data']
        js_hash = data['hash']
        controller = data['controller']

        path = self.get_script_path(js_hash, controller)

        return self.run_path(path, args)

    def run(self):
        try:
            self.con.send((1, '101::%d\n' % os.getpid()))
            data = self.con.recv()

            j.logger = logger.PatchedLogHandler(self.con)
            if 'domain' in data:
                result = self.run_with_domain_name(data)
            elif 'hash' in data:
                result = self.run_with_content(data)

            j.logger.log(json.dumps(result), j.logger.RESULT_JSON)

        except Exception as e:
            error = traceback.format_exc()
            logging.error(error)
            eco = j.errorconditionhandler.parsePythonErrorObject(e)
            # logging the critical message
            j.logger.log(eco.toJson(), j.logger.LOG_CRITICAL)
            self.con.send((0, e))
        finally:
            self.con.send((0, StopIteration()))
            self.con.close()


class CleanerThread(Process):
    def __init__(self, path, **kwargs):
        super(CleanerThread, self).__init__(**kwargs)
        self.path = path

    def run(self):
        while True:
            try:
                now = time.time()
                for fname in j.sal.fs.listFilesInDir(self.path, filter='*.js'):
                    logging.info('Checking file for clean up %s' % fname)
                    mtime = os.path.getmtime(fname)
                    if now - mtime >= SCRIPTS_DELETE_OLDER_THAN:
                        logging.info('Deleting old file %s' % fname)
                        j.sal.fs.remove(fname)
                        j.sal.fs.remove('%sc' % fname)
            except Exception as e:
                # never die. just log error.
                logging.log('Error while cleaning up old scripts %s' % e)
            finally:
                time.sleep(3600)  # run every hour.


def poolWork(con):
    WrapperThread(con).run()


def daemon(data):
    assert 'SOCKET' in os.environ, 'SOCKET env var is not set'
    assert 'JUMPSCRIPTS_HOME' in os.environ, 'JUMPSCRIPTS_HOME env var is not set'

    pid = os.getpid()

    logging.basicConfig(format=LOG_FORMAT,
                        level=logging.INFO)

    logging.info("Starting daemon '%s'" % pid)

    # make sure you create the pool first thing otherwise, the behavior can be undefined.
    pool = Pool(POOL_SIZE, maxtasksperchild=MAX_JOBS_PER_WORKER)

    # make sure we create cache folder.
    j.sal.fs.createDir(SCRIPTS_CACHE_DIR)

    # starting clean up thread.
    cleaner = CleanerThread(SCRIPTS_CACHE_DIR, name='Cleaner Thread')
    logging.info('Starting the clean up thread')
    cleaner.start()

    unix_sock_path = os.environ.get('SOCKET')
    try:
        os.unlink(unix_sock_path)
    except:
        pass

    try:
        listener = connection.Listener(address=unix_sock_path)

    except Exception as e:
        raise e

    def terminate(n, f):
        logging.info('Stopping daemon %s: %s' % (n, pid))
        listener.close()
        cleaner.terminate()
        pool.close()
        logging.info('Exit %s' % pid)
        # Suicide, please don't remove, I wanna DIE!
        # Reasons: it seems this is the only way to terminated this process for some reason. The pool doesn't
        # want to terminate nicely.
        os.kill(pid, signal.SIGKILL)

    for s in (signal.SIGTERM, signal.SIGHUP, signal.SIGQUIT, signal.SIGINT):
        signal.signal(s, terminate)

    while True:
        try:
            con = listener.accept()
            pool.apply_async(poolWork, args=(con,))
        except Exception as e:
            logging.error(e)

utils.run(daemon)
