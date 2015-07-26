import sys
import json


def result(data, format='json'):
    assert format == 'json', 'Not supported format, only json is supported so far'

    sys.stdout.write('20:::')
    sys.stdout.write(json.dumps(data))
    sys.stdout.write('\n:::')

    sys.stdout.flush()


def run(func):
    rawin = sys.stdin.read()
    rawin = rawin.strip()

    data = None
    if rawin:
        data = json.loads(rawin)

    result(func(data))
