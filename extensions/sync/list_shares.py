import utils

import api
import _sync as sync


INTERNAL = ('/opt/jumpscale7/apps/agent2/jumpscripts',)


def list_shares(data):
    syncthing = api.Syncthing(sync.SYNCTHING_URL)

    config = syncthing.config

    folders = config['folders']

    results = []
    for folder in folders:
        if folder['path'] in INTERNAL:
            continue
        results.append(folder)

    return results

if __name__ == '__main__':
    utils.run(list_shares)
