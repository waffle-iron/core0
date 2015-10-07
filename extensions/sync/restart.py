import utils
import api

import _sync as sync


def sync_folder(data):
    syncthing = api.Syncthing(sync.SYNCTHING_URL)
    response = syncthing.post(api.ENDPOINT_RESTART)

    if not response.ok:
        raise Exception('Failed to restart syncthing: %s' % response.reason)

if __name__ == '__main__':
    utils.run(sync_folder)
