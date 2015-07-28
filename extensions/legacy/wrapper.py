import utils
import imp
import os

def wrapper(data):
    path = '{js_domain}/{js_name}.py'.format(**data)
    data.pop('js_domain')
    data.pop('js_name')
    module = imp.load_source(path, path)
    return module.action(**data)

utils.run(wrapper)
