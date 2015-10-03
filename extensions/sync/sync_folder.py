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
            seconds = 3 * _errors
            logging.info('Error retreiving syncthing config, retrying in %s seconds', seconds)
            time.sleep(seconds)

    config = response.json()

    local_device_id = response.headers['x-syncthing-id']
    # Get API key for future use
    api_key = config['gui']['apiKey']
    headers['X-API-Key'] = api_key

    remote_device_id = data['device_id']
    remote_device_address = data.get('address', 'dynamic')
    devices = filter(lambda d: d['deviceID'] == remote_device_id, config['devices'])

    dirty = False
    if not devices:
        device = {
            'addresses': [remote_device_address],
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

    folder_path = os.path.join(sync.settings['agent-home'], data['path'])
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
            'path': folder_path,
            'pullers': 16,
            'readOnly': False,
            'rescanIntervalS': 60,
            'versioning': {'params': {}, 'type': ''}
        }

        if not os.path.isdir(folder_path):
            os.makedirs(folder_path, 0755)

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


if __name__ == '__main__':
    utils.run(sync_folder)
