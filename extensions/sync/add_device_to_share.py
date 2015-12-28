import utils
import api

import _sync as sync


def validate_data(data):
    for key in ('device_id', 'folder_id'):
        if key not in data:
            raise ValueError('Invalid request data')


def add_device_to_share(data):
    validate_data(data)

    syncthing = api.Syncthing(sync.SYNCTHING_URL)

    config = syncthing.config

    remote_device_id = data['device_id']
    folder_id = data['folder_id']

    # add device to shared folder.
    folders = list(filter(lambda f: f['id'] == folder_id, config['folders']))

    if not folders:
        raise Exception('No share with path=%s' % data['path'])

    folder = folders[0]

    dirty = False
    if not list(filter(lambda d: d['deviceID'] == remote_device_id, folder['devices'])):
        # share folder with device.

        folder['devices'].append({
            'deviceID': remote_device_id
        })
        dirty = True

    if not dirty:
        return

    syncthing.set_config(config)
    syncthing.restart()


if __name__ == '__main__':
    utils.run(add_device_to_share)
