import utils
import _sync as sync

import api


def get_id(data):
    syncthing = api.Syncthing(sync.SYNCTHING_URL)

    return syncthing.device_id

if __name__ == '__main__':
    utils.run(get_id)
