import utils
import os

import api
import _sync as sync

agent_home = os.environ.get('AGENT_HOME')

INTERNAL = ('%s/agent8/jumpscripts' % agent_home,)


def list_shares(data):
    syncthing = api.Syncthing(sync.SYNCTHING_URL)

    config = syncthing.config

    folders = config['folders']

    results = []
    for folder in folders:
        skip = False
        for internal_path in INTERNAL:
            if folder['path'].startswith(internal_path):
                skip = True
                break

        if skip:
            continue

        results.append(folder)

    return results

if __name__ == '__main__':
    utils.run(list_shares)
