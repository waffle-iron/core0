import utils
import api

import _sync as sync


def restart(data):
    syncthing = api.Syncthing(sync.SYNCTHING_URL)
    syncthing.restart()

if __name__ == '__main__':
    utils.run(restart)
