import utils
import api

import _sync as sync


def validate_data(data):
    for key in ('id', 'name'):
        if key not in data:
            raise ValueError('Invalid request data')


def add_device(data):
    validate_data(data)

    syncthing = api.Syncthing(sync.SYNCTHING_URL)

    config = syncthing.config

    remote_device_id = data['id']
    remove_device_name = data['name']
    remote_device_address = data.get('address', 'dynamic')

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

    if not dirty:
        return

    syncthing.set_config(config)
    syncthing.restart()


if __name__ == '__main__':
    utils.run(add_device)
