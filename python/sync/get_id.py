import utils
import requests

import sync

ENDPOINT = '/rest/system/status'


def get_id(data):
    url = '{host}/{endpoint}'.format(
        host=sync.settings['host'].rstrip('/'),
        endpoint=ENDPOINT
    )

    response = requests.get(url)
    if not response.ok:
        raise Exception('Invalid response from syncthing: %s' % response.reason)

    result = response.json()

    return result['myID']


utils.run(get_id)
