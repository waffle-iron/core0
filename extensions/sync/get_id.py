import utils
import requests
import logging
import time
import _sync as sync


def get_id(data):
    url = sync.get_url(sync.ENDPOINT_STATUS)

    _errors = 0
    while True:
        try:
            response = requests.get(url)
            if not response.ok:
                raise Exception('Invalid response from syncthing: %s' % response.reason)
            else:
                break
        except:
            _errors += 1
            if _errors >= 3:
                raise
            seconds = 3 * _errors
            logging.info('Error retreiving syncthing config, retrying in %s seconds', seconds)
            time.sleep(seconds)

    result = response.json()

    return result['myID']

if __name__ == '__main__':
    utils.run(get_id)
