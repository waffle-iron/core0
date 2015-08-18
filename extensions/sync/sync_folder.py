import os
import utils
import requests
import json
import time
import logging

import _sync as sync


def validate_data(data):
    for key in ('device_id', 'folder_id', 'path'):
        if key not in data:
            raise ValueError('Invalid request data')


def sync_folder(data):
    validate_data(data)

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
            logging.info('Error retreiving syncthing config, retrying in 3 seconds')
            time.sleep(3)

    config = response.json()

    local_device_id = response.headers['x-syncthing-id']
    # Get API key for future use
    api_key = config['gui']['apiKey']
    headers['X-API-Key'] = api_key

    remote_device_id = data['device_id']
    devices = filter(lambda d: d['deviceID'] == remote_device_id, config['devices'])

    dirty = False
    if not devices:
        device = {
            'addresses': ['dynamic'],
            'certName': '',
            'compression': 'metadata',
            'deviceID': remote_device_id,
            'introducer': False,
            'name': remote_device_id.split('-')[0]
        }

        config['devices'].append(device)
        dirty = True

    # add device to shared folder.
    folders = filter(lambda f: f['id'] == data['folder_id'], config['folders'])

    if not folders:
        # add folder.
        folder = {
            'autoNormalize': False,
            'copiers': 1,
            'devices': [{'deviceID': local_device_id}],
            'hashers': 0,
            'id': data['folder_id'],
            'ignorePerms': False,
            'invalid': '',
            'order': 'random',
            'path': data['path'],
            'pullers': 16,
            'readOnly': False,
            'rescanIntervalS': 60,
            'versioning': {'params': {}, 'type': ''}
        }

        if not os.path.isdir(data['path']):
            os.makedirs(data['path'], 0755)

        config['folders'].append(folder)
        dirty = True
    else:
        folder = folders[0]

    if not filter(lambda d: d['deviceID'] == remote_device_id, folder['devices']):
        # share folder with device.

        folder['devices'].append({
            'deviceID': remote_device_id
        })
        dirty = True

    if not dirty:
        return

    response = sessions.post(config_url, data=json.dumps(config), headers=headers)
    if not response.ok:
        raise Exception('Failed to set syncthing configuration', response.reason)

    response = sessions.post(sync.get_url(sync.ENDPOINT_RESTART), headers=headers)
    if not response.ok:
        raise Exception('Failed to restart syncthing', sync.get_url(sync.ENDPOINT_RESTART), response.reason)

if __name__ == '__main__':
    utils.run(sync_folder)
