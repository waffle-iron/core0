import redis
import uuid
import json
import textwrap
import shlex


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
        if v is not None:
            payload = json.loads(v.decode())
            return Return(payload)
        return None


class BaseClient:
    def __init__(self, gid, nid):
        self._gid = gid
        self._nid = nid

    def raw(self, command, arguments):
        raise NotImplemented()

    def system(self, command, dir='', stdin='', env=None):
        parts = shlex.split(command)
        if len(parts) == 0:
            raise ValueError('invalid command')

        response = self.raw(command='system', arguments={
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
            raise RuntimeError('failed to dispatch command to container: %s', result.data)

        cmd_id = json.loads(result.data)
        return self._client.response_for(cmd_id)


class ContainerManager:
    def __init__(self, client):
        self._client = client

    def create(self, plist_url):
        response = self._client.raw('corex.create', {
            'plist': plist_url,
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
            raise RuntimeError('failed to list containers: %s' % result.data)

    def client(self, container):
        return ContainerClient(self._client, container)


class Client(BaseClient):
    def __init__(self, gid, nid, host="localhost", port=6379, password="", db=0):
        super().__init__(gid, nid)

        self._redis = redis.Redis(host=host, port=port, password=password, db=db)
        self._container_manager = ContainerManager(self)

    @property
    def container(self):
        return self._container_manager

    def raw(self, command, arguments):
        id = str(uuid.uuid4())

        payload = {
            'id': id,
            'command': command,
            'arguments': arguments,
        }

        queue = 'core:default:{}:{}'.format(self._gid, self._nid)
        self._redis.rpush(queue, json.dumps(payload))

        return Response(self, id)

    def bash(self, command):
        response = self.raw(command='bash', arguments={
            'stdin': command,
        })

        return response

    def response_for(self, id):
        return Response(self, id)
