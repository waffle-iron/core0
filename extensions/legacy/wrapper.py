import utils
import os
import imp


def wrapper(data):
    jspath = os.environ.get('JUMPSCRIPTS_HOME')
    if jspath is None:
        raise RuntimeError('Missconfigured, no jspath define')

    path = os.path.join(jspath, data['domain'], '%s.py' % data['name'])
    module = imp.load_source(path, path)
    return module.action(**data['args'])

utils.run(wrapper)
