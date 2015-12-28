import requests
import logging
import time
import json
from hashlib import md5

ENDPOINT_CONFIG = '/rest/system/config'
ENDPOINT_RESTART = '/rest/system/restart'
ENDPOINT_STATUS = '/rest/system/status'
ENDPOINT_SCAN = '/rest/db/scan'


class Syncthing(object):
    def __init__(self, base_url):
        self.base_url = base_url
        self._headers = None
        self._device_id = None
        self._config = None
        self.init()

    @property
    def headers(self):
        return self._headers

    @property
    def device_id(self):
        return self._device_id

    @property
    def config(self):
        return self._config

    def get_url(self, endpoint):
        syncthing = self.base_url.rstrip('/')

        url = '{host}{endpoint}'.format(
            host=syncthing,
            endpoint=endpoint
        )

        return url

    def init(self):
        # loads the config and the API key.
        sessions = requests.Session()

        headers = {
            'content-type': 'application/json'
        }

        config_url = self.get_url(ENDPOINT_CONFIG)

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
                seconds = 5 * _errors
                logging.info('Error retreiving syncthing config, retrying in %s seconds', seconds)
                time.sleep(seconds)

        config = response.json()

        device_id = response.headers['x-syncthing-id']
        # Get API key for future use
        api_key = config['gui']['apiKey']
        headers['X-API-Key'] = api_key

        self._sessions = sessions
        self._headers = headers
        self._device_id = device_id
        self._config = config

    def post(self, endpoint, data=None):
        url = self.get_url(endpoint)
        data = json.dumps(data) if data is not None else None
        return self._sessions.post(url, data=data, headers=self.headers)

    def get(self, endpoint, data=None):
        url = self.get_url(endpoint)
        return self._sessions.get(url, params=data, headers=self.headers)

    def set_config(self, config):
        response = self.post(ENDPOINT_CONFIG, data=config)
        if not response.ok:
            raise Exception('Failed to set syncthing configuration', response.reason)

    def restart(self):
        response = self.post(ENDPOINT_RESTART)

        if not response.ok:
            raise Exception('Failed to restart syncthing: %s' % response.reason)

    def folder_path_to_id(self, path):
        return md5(path).hexdigest()

    def scan(self, folder, sub=None):
        data = {'folder': folder}
        if sub:
            data['sub'] = sub
        response= self.post(ENDPOINT_SCAN, data)
        if not response.ok:
            raise Exception('Failed to scan folder %s' % folder, response.reason)
