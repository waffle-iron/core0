import utils
from sync_folder import sync_folder
import _sync as sync


def sync_jumpscripts(data):
    data.update({
        'path': sync.settings['jumpscripts-path'],
    })

    sync_folder(data)


if __name__ == '__main__':
    utils.run(sync_jumpscripts)
