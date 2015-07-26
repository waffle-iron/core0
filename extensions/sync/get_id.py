import utils
import requests

import _sync as sync


def get_id(data):
    url = sync.get_url(sync.ENDPOINT_STATUS)

    response = requests.get(url)
    if not response.ok:
        raise Exception('Invalid response from syncthing: %s' % response.reason)

    result = response.json()

    return result['myID']

if __name__ == '__main__':
    utils.run(get_id)
