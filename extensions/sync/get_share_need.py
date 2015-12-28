import utils

import api
import _sync as sync


def validate_data(data):
    for key in ('name',):
        if key not in data:
            raise ValueError('Invalid request data, missing "%s"' % key)


def get_ignore(data):
    validate_data(data)

    syncthing = api.Syncthing(sync.SYNCTHING_URL)

    config = syncthing.config

    # add device to shared folder.
    folders = list(filter(lambda f: f['id'] == data['name'], config['folders']))

    if not folders:
        raise Exception('No share with name=%s' % data['name'])
    else:
        folder = folders[0]

    response = syncthing.get('/rest/db/need', {'folder': folder['id']})

    if not response.ok:
        raise Exception('Could not get share ignore list: %s' % response.reason)

    return response.json()


if __name__ == '__main__':
    utils.run(get_ignore)
