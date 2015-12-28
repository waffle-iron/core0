import utils

import api
import _sync as sync


def validate_data(data):
    for key in ('path',):
        if key not in data:
            raise ValueError('Invalid request data, missing "%s"' % key)


def get_ignore(data):
    validate_data(data)

    syncthing = api.Syncthing(sync.SYNCTHING_URL)

    config = syncthing.config

    # add device to shared folder.
    folders = list(filter(lambda f: f['path'] == data['path'], config['folders']))

    if not folders:
        raise Exception('No share with path=%s' % data['path'])
    else:
        folder = folders[0]

    response = syncthing.get('/rest/db/ignores', {'folder': folder['id']})

    if not response.ok:
        raise Exception('Could not get share ignore list: %s' % response.reason)

    return response.json()


if __name__ == '__main__':
    utils.run(get_ignore)
