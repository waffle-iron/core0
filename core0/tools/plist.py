import os
import re
import subprocess


TYPE_FILE = 2
TYPE_DIR = 4


class Entry:
    """
    filepath|hash|filesize|uname|gname|permissions|filetype|ctime|mtime|extended
    """
    def __init__(self, path, hash, size, filetype, uname=0, gname=0, permissions=0o755, ctime=0, mtime=0, extended=''):
        self._data = {
            'path': path,
            'hash': hash,
            'size': size,
            'uname': uname,
            'gname': gname,
            'permissions': permissions,
            'filetype': filetype,
            'ctime': ctime,
            'mtime': mtime,
            'extended': extended,
        }

    def trim(self, base):
        path = self._data['path']
        if path.rfind(base) == 0:
            path = path[len(base):]
            self._data['path'] = path

    def __str__(self):
        return '{path}|{hash}|{size}|{uname}|{gname}|{permissions}|{filetype}|{ctime}|{mtime}|{extended}'.format(
            **self._data)


def generate(root):
    for root, dirs, files in os.walk(root):
        if not dirs and not files:
            # empty directory
            state = os.stat(root)
            yield Entry(root, '', 0, TYPE_DIR, state.st_uid, state.st_gid,
                        '%o' % (state.st_mode & 0o000777), int(state.st_ctime), int(state.st_mtime))
        for name in files:
            path = os.path.join(root, name)
            cmd = 'brotli --input {} | ipfs add'.format(path)
            out = subprocess.run(cmd, shell=True, stdout=subprocess.PIPE)
            m = re.match('^added .+ (.+)$', out.stdout.decode())
            if m is None:
                raise RuntimeError('invalid output from ipfs add: %s' % out)
            hash = m.group(1)

            state = os.stat(path)
            yield Entry(path, hash, state.st_size, TYPE_FILE, state.st_uid, state.st_gid,
                        '%o' % (state.st_mode & 0o000777), int(state.st_ctime), int(state.st_mtime))

if __name__ == '__main__':
    root = 'chroot'
    for entry in generate(root):
        entry.trim(root)
        print(entry)

    # with open('plist', 'w') as f:
    #     for entry in generate('/root/to/tree'):
    #         f.write('%s\n' % entry)

