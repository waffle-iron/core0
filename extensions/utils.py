import sys
import json
import functools
import fcntl
import os


L_STDOUT = 1  # stdout
L_STDERR = 2  # stderr
L_PUBLIC = 3  # message for endusers / public message
L_OPERATOR = 4  # message for operator / internal message
L_UNKNOWN = 5  # log msg (unstructured = level5, cat=unknown)
L_STRUCTURED = 6  # log msg structured
L_WARNING = 7  # warning message
L_OPS_ERROR = 8  # ops error
L_CRITICAL = 9  # critical error
L_STATSD = 10  # statsd message(s) AVG
L_RESULT_JSON = 20  # result message, json
L_RESULT_YAML = 21  # result message, yaml
L_RESULT_TOML = 22  # result message, toml
L_RESULT_HRD = 23  # result message, hrd
L_RESULT_JOB = 30  # job, json (full result of a job)


def log(msg, level=L_STDOUT):
    if os.linesep in msg:
        sys.stdout.write('%d:::' % level)
        sys.stdout.write(msg)
        sys.stdout.write('%s:::' % os.linesep)
    else:
        sys.stdout.write('%d::' % level)
        sys.stdout.write(msg)
        sys.stdout.write(os.linesep)


def result(data, format='json'):
    assert format == 'json', 'Not supported format, only json is supported so far'

    sys.stdout.write('20:::')
    sys.stdout.write(json.dumps(data))
    sys.stdout.write('\n:::')

    sys.stdout.flush()


def run(func):
    """
    Runs func and then exit with exit code 0. Not this function never returns

    :param func: a function to run with signature `def func(data)`
    """
    result(dryrun(func))
    sys.exit(0)


def dryrun(func):
    """
    Runs `func` and feeds it stadin as data

    :param func: a function to run with signature `def func(data)`

    :rtype: object
    """
    rawin = sys.stdin.read()
    rawin = rawin.strip()

    data = None
    if rawin:
        data = json.loads(rawin)

    return func(data)


class Lock(object):

    def __init__(self, path):
        self._path = path

    def __enter__(self):
        self._fd = open(self._path, 'w')
        fcntl.flock(self._fd, fcntl.LOCK_EX)

    def __exit__(self, type, value, traceback):
        fcntl.flock(self._fd, fcntl.LOCK_UN)
        self._fd.close()


class exclusive(object):  # NOQA

    def __init__(self, path):
        self._path = path

    def __call__(self, fnc):
        @functools.wraps(fnc)
        def wrapper(*args, **kwargs):
            with Lock(self._path):
                return fnc(*args, **kwargs)

        return wrapper
