import utils

import api
import _sync as sync


def list_devices(data):
    syncthing = api.Syncthing(sync.SYNCTHING_URL)

    config = syncthing.config

    return config['devices']


if __name__ == '__main__':
    utils.run(list_devices)
