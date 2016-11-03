import redis
import uuid
import json


class Response:
    def __init__(self, client, id):
        self._client = client
        self._queue = 'result:{}'.format(id)

    def get(self, timeout=10):
        r = self._client._redis
        v = r.brpoplpush(self._queue, self._queue, timeout)
        if v is not None:
            return json.loads(v.decode())
        return None


class Client:
    def __init__(self, host="localhost", port=6379, password="", db=0):
        self._redis = redis.Redis(host=host, port=port, password=password, db=db)

    def raw(self, gid, nid, command, arguments):
        id = str(uuid.uuid4())

        payload = {
            'id': id,
            'command': command,
            'arguments': arguments,
        }

        queue = 'core:default:{}:{}'.format(gid, nid)
        self._redis.rpush(queue, json.dumps(payload))

        return Response(self, id)

    def response_for(self, id):
        return Response(self, id)

