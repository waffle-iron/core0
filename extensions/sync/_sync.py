import os

settings = {
    'syncthing': os.environ.get('SYNCTHING_URL', 'http://localhost:8384'),
    'agent-home': os.environ.get('AGENT_HOME'),
    'controller-name': os.environ.get('AGENT_CONTROLLER_NAME', 'unknown')
}

ENDPOINT_CONFIG = '/rest/system/config'
ENDPOINT_RESTART = '/rest/system/restart'
ENDPOINT_STATUS = '/rest/system/status'

SYNCTHING_URL = settings['syncthing']


def get_url(endpoint):
    syncthing = settings['syncthing'].rstrip('/')

    url = '{host}{endpoint}'.format(
        host=syncthing,
        endpoint=endpoint
    )
    return url
