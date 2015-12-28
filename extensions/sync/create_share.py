import os
import utils
import urllib

import api
import _sync as sync


def validate_data(data):
    for key in ('name', 'path', 'ignore', 'readonly'):
        if key not in data:
            raise ValueError('Invalid request data, missing "%s"' % key)


def create_share(data):
    validate_data(data)

    syncthing = api.Syncthing(sync.SYNCTHING_URL)

    config = syncthing.config

    # add device to shared folder.
    folders = list(filter(lambda f: f['path'] == data['path'], config['folders']))

    folder_id = data['name']
    folder_path = os.path.join(sync.settings['agent-home'], data['path'])
    dirty = False
    if not folders:
        # add folder.
        folder = {
            'autoNormalize': False,
            'copiers': 1,
            'devices': [{'deviceID': syncthing.device_id}],
            'hashers': 0,
            'id': folder_id,
            'ignorePerms': False,
            'invalid': '',
            'order': 'random',
            'path': folder_path,
            'pullers': 16,
            'readOnly': data['readonly'],
            'rescanIntervalS': 60,
            'versioning': {'params': {}, 'type': ''}
        }

        if not os.path.isdir(folder_path):
            os.makedirs(folder_path, 0o755)

        config['folders'].append(folder)
        dirty = True
    else:
        folder = folders[0]

    if dirty:
        syncthing.set_config(config)

    ignore = data['ignore'] or []
    if ignore:
        ignore_url = '/rest/db/ignores?%s' % urllib.urlencode({'folder': folder_id})
        syncthing.post(ignore_url, {'ignore': ignore})

    syncthing.restart()

    return folder


if __name__ == '__main__':
    utils.run(create_share)
