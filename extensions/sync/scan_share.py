import utils
import api

import _sync as sync


def scan_folder(data):
    syncthing = api.Syncthing(sync.SYNCTHING_URL)
    syncthing.scan(data['name'], data['sub'])

if __name__ == '__main__':
    utils.run(scan_folder)
