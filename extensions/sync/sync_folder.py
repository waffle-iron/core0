import os
import utils
import api

import _sync as sync


def validate_data(data):
    for key in ('device_id', 'folder_id', 'path'):
        if key not in data:
            raise ValueError('Invalid request data')


def sync_folder(data):
    """
    This action is only (and should be only) used by the agent controller to setup the jumpscripts synching
    """
    validate_data(data)

    syncthing = api.Syncthing(sync.SYNCTHING_URL)

    config = syncthing.config

    remote_device_id = data['device_id']
    remote_device_address = data.get('address', 'dynamic')
    remove_device_name = data.get('name', remote_device_id.split('-')[0])
    devices = list(filter(lambda d: d['deviceID'] == remote_device_id, config['devices']))

    dirty = False
    if not devices:
        device = {
            'addresses': [remote_device_address],
            'certName': '',
            'compression': 'metadata',
            'deviceID': remote_device_id,
            'introducer': False,
            'name': remove_device_name
        }

        config['devices'].append(device)
        dirty = True
    else:
        device = devices[0]
        # Compare address, in case device changed it's address
        if device['addresses'][0] != remote_device_address:
            device['addresses'] = [remote_device_address]
            dirty = True

    # add device to shared folder.
    folders = list(filter(lambda f: f['id'] == data['folder_id'], config['folders']))

    folder_path = os.path.join(sync.settings['agent-home'], data['path'], sync.settings['controller-name'])
    if not folders:
        # add folder.
        folder = {
            'autoNormalize': False,
            'copiers': 1,
            'devices': [{'deviceID': syncthing.device_id}],
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
            os.makedirs(folder_path, 0o755)

        config['folders'].append(folder)
        dirty = True
    else:
        folder = folders[0]

    if not list(filter(lambda d: d['deviceID'] == remote_device_id, folder['devices'])):
        # share folder with device.

        folder['devices'].append({
            'deviceID': remote_device_id
        })
        dirty = True

    if not dirty:
        return

    syncthing.set_config(config)


if __name__ == '__main__':
    utils.run(sync_folder)
