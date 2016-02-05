import io
import collections
import os

from JumpScale.core.logging.LogHandler import LogHandler

LEVEL = collections.namedtuple('LEVEL', 'name level')
STATS = collections.namedtuple('STATS', 'name op')

LEGACY_LOG_UNKNOWN = 0
LEGACY_LOG_ENDUSER_MSG = 1
LEGACY_LOG_OPERATOR_MSG = 2
LEGACY_LOG_STDOUT = 3
LEGACY_LOG_STDERR = 4
LEGACY_LOG_TRACING_1 = 5
LEGACY_LOG_TRACING_2 = 6
LEGACY_LOG_TRACING_3 = 7
LEGACY_LOG_TRACING_4 = 8
LEGACY_LOG_TRACING_5 = 9
LEGACY_LOG_MARKER = 10


class PatchedLogHandler(LogHandler):
    """
    A LogHandler patch that implements agent logging interface.

    :param con: the :class:`multiprocessing.Connection` object
    """

    LOG_STDOUT = LEVEL('STDOUT', 1)
    LOG_STDERR = LEVEL('STDERR', 2)
    LOG_PUBLIC = LEVEL('PUBLIC', 3)
    LOG_OPERATOR = LEVEL('OPERATOR', 4)
    LOG_UNKNOWN = LEVEL('UNKNOWN', 5)
    LOG_STRUCTURED = LEVEL('STRUCTURED', 6)
    LOG_WARNING = LEVEL('WARNING', 7)
    LOG_OPS_ERROR = LEVEL('OPS_ERROR', 8)
    LOG_CRITICAL = LEVEL('CRITICAL', 9)
    LOG_STATSD = LEVEL('STATSD', 10)
    LOG_DEBUG = LEVEL('STATSD', 11)

    STATS_KEYVALUE = STATS('KEYVALUE', 'kv')
    STATS_GAUAGE = STATS('GAUAGE', 'g')
    STATS_TIMER = STATS('TIMER', 'ms')
    STATS_COUNTER = STATS('COUNTER', 'c')
    STATS_UNIQUESET = STATS('UNIQUESET', 's')

    RESULT_JSON = LEVEL('JSON', 20)
    RESULT_YAML = LEVEL('YAML', 21)
    RESULT_TOML = LEVEL('TOML', 22)
    RESULT_HRD = LEVEL('HRD', 23)
    RESULT_JOB = LEVEL('JOB', 30)

    _LMAP = {
        LEGACY_LOG_UNKNOWN: LOG_UNKNOWN,
        LEGACY_LOG_ENDUSER_MSG: LOG_STDOUT,
        LEGACY_LOG_OPERATOR_MSG: LOG_OPERATOR,
        LEGACY_LOG_STDOUT: LOG_STDOUT,
        LEGACY_LOG_STDERR: LOG_STDERR,
        LEGACY_LOG_TRACING_1: LOG_DEBUG,
        LEGACY_LOG_TRACING_2: LOG_DEBUG,
        LEGACY_LOG_TRACING_3: LOG_DEBUG,
        LEGACY_LOG_TRACING_4: LOG_DEBUG,
        LEGACY_LOG_TRACING_5: LOG_DEBUG,
        LEGACY_LOG_MARKER: LOG_UNKNOWN,
    }

    def __init__(self, con):
        super(PatchedLogHandler, self).__init__()
        self._con = con

    def log(self, msg, level=LOG_DEBUG, **kwargs):
        """

        :param msg: The message to log
        :param level: The message level (defeault to stdout)
        :param kwargs: Eats up all remaning kwargs for compatibility with
                     The normal logger
        """

        num_level = PatchedLogHandler.LOG_UNKNOWN.level

        if isinstance(level, LEVEL):
            num_level = level.level
        elif isinstance(level, int):
            num_level = PatchedLogHandler._LMAP.get(level, PatchedLogHandler.LOG_UNKNOWN).level
        else:
            raise ValueError('Unknown log level type')

        multiline = os.linesep in msg

        buff = io.StringIO()

        if multiline:
            buff.write('%d:::' % num_level)
            buff.write(msg)
            buff.write('\n:::')
        else:
            buff.write('%d::%s\n' % (num_level, msg))

        # send to stdout.
        self._con.send((1, buff.getvalue()))

    def stats(self, key, value, op=STATS_GAUAGE):
        self.log('%s:%g|%s' % (key, value, op.op), PatchedLogHandler.LOG_STATSD)


class StreamHandler(io.RawIOBase):
    def __init__(self, con, default_level):
        self._con = con
        self._default_level = default_level

    def write(self, b):
        self._con.send((self._default_level, b))
