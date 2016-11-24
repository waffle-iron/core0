import redis
import uuid
import json
import textwrap
import shlex


class Timeout(Exception):
    pass


class Return:
    def __init__(self, payload):
        self._payload = payload

    @property
    def payload(self):
        return self._payload

    @property
    def id(self):
        return self._payload['id']

    @property
    def data(self):
        """
        data returned by the process. Only available if process
        output data with the correct core level
        """
        return self._payload['data']

    @property
    def level(self):
        """data message level (if any)"""
        return self._payload['level']

    @property
    def starttime(self):
        """timestamp"""
        return self._payload['starttime'] / 1000

    @property
    def time(self):
        """execution time in millisecond"""
        return self._payload['time']

    @property
    def state(self):
        """
        exit state
        """
        return self._payload['state']

    @property
    def stdout(self):
        streams = self._payload.get('streams', None)
        return streams[0] if streams is not None and len(streams) >= 1 else ''

    @property
    def stderr(self):
        streams = self._payload.get('streams', None)
        return streams[1] if streams is not None and len(streams) >= 2 else ''

    def __repr__(self):
        return str(self)

    def __str__(self):
        tmpl = """\
        STATE: {state}
        STDOUT:
        {stdout}
        STDERR:
        {stderr}
        DATA:
        {data}
        """

        return textwrap.dedent(tmpl).format(state=self.state, stdout=self.stdout, stderr=self.stderr, data=self.data)


class Response:
    def __init__(self, client, id):
        self._client = client
        self._queue = 'result:{}'.format(id)

    def get(self, timeout=10):
        r = self._client._redis
        v = r.brpoplpush(self._queue, self._queue, timeout)
        if v is None:
            raise Timeout()
        payload = json.loads(v.decode())
        return Return(payload)


class InfoManager:
    def __init__(self, client):
        self._client = client

    def cpu(self):
        return self._client.raw(command='info.cpu', arguments={})

    def nic(self):
        return self._client.raw(command='info.nic', arguments={})

    def mem(self):
        return self._client.raw(command='info.mem', arguments={})

    def disk(self):
        return self._client.raw(command='info.disk', arguments={})

    def os(self):
        return self._client.raw(command='info.os', arguments={})


class BaseClient:
    def __init__(self):
        self._info = InfoManager(self)

    @property
    def info(self):
        return self._info

    def raw(self, command, arguments):
        raise NotImplemented()

    def ping(self):
        response = self.raw('core.ping', {})

        result = response.get()
        if result.state != 'SUCCESS':
            raise RuntimeError('invalid response: %s' % result.state)

        return json.loads(result.data)

    def system(self, command, dir='', stdin='', env=None):
        parts = shlex.split(command)
        if len(parts) == 0:
            raise ValueError('invalid command')

        response = self.raw(command='core.system', arguments={
            'name': parts[0],
            'args': parts[1:],
            'dir': dir,
            'stdin': stdin,
            'env': env,
        })

        return response


class ContainerClient(BaseClient):
    def __init__(self, client, container):
        self._client = client
        self._container = container

    def raw(self, command, arguments):
        response = self._client.raw('corex.dispatch', {
            'container': self._container,
            'command': {
                'command': command,
                'arguments': arguments,
            },
        })

        result = response.get()
        if result.state != 'SUCCESS':
            raise RuntimeError('failed to dispatch command to container: %s' % result.data)

        cmd_id = json.loads(result.data)
        return self._client.response_for(cmd_id)


class ContainerManager:
    def __init__(self, client):
        self._client = client

    def create(self, plist_url, mount={}, zerotier=None, bridge=[]):
        response = self._client.raw('corex.create', {
            'plist': plist_url,
            'mount': mount,
            'network': {
                'zerotier': zerotier,
                'bridge': bridge,
            },
        })

        result = response.get()
        if result.state != 'SUCCESS':
            raise RuntimeError('failed to create container %s' % result.data)

        return json.loads(result.data)

    def list(self):
        response = self._client.raw('corex.list', {})

        result = response.get()
        if result.state != 'SUCCESS':
            raise RuntimeError('failed to list containers: %s' % result.data)

        return json.loads(result.data)

    def terminate(self, container):
        response = self._client.raw('corex.terminate', {
            'container': container,
        })

        result = response.get()
        if result.state != 'SUCCESS':
            raise RuntimeError('failed to terminate container: %s' % result.data)

    def client(self, container):
        return ContainerClient(self._client, container)


class BridgeManager:
    def __init__(self, client):
        self._client = client

    def create(self, name, hwaddr=None, network=None, settings={}):
        """
        Create a bridge with the given name, hwaddr and networking setup
        :param name: name of the bridge (must be unique)
        :param hwaddr: MAC address of the bridge. If none, a one will be created for u
        :param network: Networking mode, options are none, static, and dnsmasq
        :param settings: Networking setting, depending on the selected mode.
                        None:
                            no settings, bridge won't get any ip settings
                        static:
                            settings={'cidr': 'ip/net'}
                            bridge will get assigned the given IP address
        """
        response = self._client.raw('bridge.create', {
            'name': name,
            'hwaddr': hwaddr,
            'network': {
                'mode': network,
                'settings': settings,
            }
        })

        result = response.get()
        if result.state != 'SUCCESS':
            raise RuntimeError('failed to create bridge %s' % result.data)

        return json.loads(result.data)

    def list(self):
        response = self._client.raw('bridge.list', {})

        result = response.get()
        if result.state != 'SUCCESS':
            raise RuntimeError('failed to list bridges: %s' % result.data)

        return json.loads(result.data)

    def delete(self, bridge):
        response = self._client.raw('bridge.delete', {
            'name': bridge,
        })

        result = response.get()
        if result.state != 'SUCCESS':
            raise RuntimeError('failed to list delete: %s' % result.data)


class DiskManager:
    def __init__(self, client):
        self._client = client

    def list(self):
        """
        List available block devices
        """
        response = self._client.raw('disk.list', {})

        result = response.get()

        if result.state != 'SUCCESS':
            raise RuntimeError('failed to list disks: %s' % result.stderr)

        if result.level != 20:  # 20 is JSON output.
            raise RuntimeError('invalid response type from disk.list command')

        return json.loads(result.data)

    def mktable(self, disk, table_type='gpt'):
        """
        Make partition table on block device.
        :param disk: Full device path like /dev/sda
        :param table_type: Partition table type as accepted by parted
        """

        response = self._client.raw('disk.mktable', {
            'disk': disk,
            'table_type': table_type,
        })

        result = response.get()

        if result.state != 'SUCCESS':
            raise RuntimeError('failed to create table: %s' % result.stderr)

    def mkpart(self, disk, start, end, part_type='primary'):
        """
        Make partition on disk
        :param disk: Full device path like /dev/sda
        :param start: partition start as accepted by parted mkpart
        :param end: partition end as accepted by parted mkpart
        :param part_type: partition type as accepted by parted mkpart
        """

        response = self._client.raw('disk.mkpart', {
            'disk': disk,
            'start': start,
            'end': end,
            'part_type': part_type,
        })

        result = response.get()

        if result.state != 'SUCCESS':
            raise RuntimeError('failed to create partition: %s' % result.stderr)

    def rmpart(self, disk, number):
        """
        Remove partion from disk
        :param disk: Full device path like /dev/sda
        :param number: Partition number (starting from 1)
        """
        response = self._client.raw('disk.rmpart', {
            'disk': disk,
            'number': number,
        })

        result = response.get()

        if result.state != 'SUCCESS':
            raise RuntimeError('failed to remove partition: %s' % result.stderr)

    def mount(self, source, target, options=[]):
        """
        Mount partion on target
        :param source: Full partition path like /dev/sda1
        :param target: Mount point
        :param options: Optional mount options
        """

        if len(options) == 0:
            options = ['auto']

        response = self._client.raw('disk.mount', {
            'options': ','.join(options),
            'source': source,
            'target': target,
        })

        result = response.get()

        if result.state != 'SUCCESS':
            raise RuntimeError('failed to mount partition: %s' % result.stderr)

    def umount(self, source):
        """
        Unmount partion
        :param source: Full partition path like /dev/sda1
        """

        response = self._client.raw('disk.umount', {
            'source': source,
        })

        result = response.get()

        if result.state != 'SUCCESS':
            raise RuntimeError('failed to umount partition: %s' % result.stderr)


class Client(BaseClient):
    def __init__(self, host, port=6379, password="", db=0):
        super().__init__()

        self._redis = redis.Redis(host=host, port=port, password=password, db=db)
        self._container_manager = ContainerManager(self)
        self._bridge_manager = BridgeManager(self)
        self._disk_manager = DiskManager(self)

    @property
    def container(self):
        return self._container_manager

    @property
    def bridge(self):
        return self._bridge_manager

    @property
    def disk(self):
        return self._disk_manager

    def raw(self, command, arguments):
        id = str(uuid.uuid4())

        payload = {
            'id': id,
            'command': command,
            'arguments': arguments,
        }

        self._redis.rpush('core:default', json.dumps(payload))

        return Response(self, id)

    def bash(self, command):
        response = self.raw(command='bash', arguments={
            'stdin': command,
        })

        return response

    def response_for(self, id):
        return Response(self, id)
