import utils

import api
import _sync as sync


def list_shares(data):
    syncthing = api.Syncthing(sync.SYNCTHING_URL)

    config = syncthing.config

    folders = []
    for folder in config['folders']:
        folder_id = folder['id']
        if len(folder_id) != 32:
            # expecting a md5 hexdigest
            continue

        try:
            int(folder_id, 16)
        except ValueError:
            continue

        folders.append(folder)

    return folders


if __name__ == '__main__':
    utils.run(list_shares)
