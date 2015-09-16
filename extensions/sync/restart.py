import utils
import requests
import time
import logging

import _sync as sync


def sync_folder(data):
    sessions = requests.Session()

    headers = {
        'content-type': 'application/json'
    }

    config_url = sync.get_url(sync.ENDPOINT_CONFIG)

    _errors = 0
    while True:
        try:
            response = sessions.get(config_url)
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

    config = response.json()

    # Get API key for future use
    api_key = config['gui']['apiKey']
    headers['X-API-Key'] = api_key

    # we depend on the agent to restart built in synchthing server on shutdown.
    response = sessions.post(sync.get_url(sync.ENDPOINT_RESTART), headers=headers)
    if not response.ok:
        raise Exception('Failed to restart syncthing', sync.get_url(sync.ENDPOINT_RESTART), response.reason)

if __name__ == '__main__':
    utils.run(sync_folder)
