import os

settings = {
    'syncthing': os.environ.get('SYNCTHING_URL', 'http://localhost:8384'),
    'jumpscripts-path': os.path.abspath(os.environ['JUMPSCRIPTS_HOME'])
}

ENDPOINT_CONFIG = '/rest/system/config'
ENDPOINT_RESTART = '/rest/system/restart'
ENDPOINT_STATUS = '/rest/system/status'


def get_url(endpoint):
    syncthing = settings['syncthing'].rstrip('/')

    url = '{host}{endpoint}'.format(
        host=syncthing,
        endpoint=endpoint
    )
    return url
