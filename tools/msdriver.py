# This test drivee script will do the following:
# 1- Create a master machine on ms1 with jumpscale and use this machine to build both
#    agent8 and agentcontroller8 and run the go tests
# 2- If passed, first step it will run an agent-controller on the master machine
# 3- It will create 3 more machines on ms1 and install the agent8 (using ays) on the
#    3 machines, giving them Node-Ids from 1, to 3
# 4- Run a simply get_os_info querey on the 3 agents, and make sure that the 3 of them are
#    alive and responding with the corrent data

from JumpScale import j

from fabric.context_managers import shell_env

# create the vn on ms1
data = {
    'instance.param.name': 'agent8_master',
    'instance.param.memsize': 1,
    'instance.param.ssdsize': 10,
    'instance.param.imagename': 'ubuntu.14.04.x64',
    'instance.paran.ssh.shell': '/bin/bash -l -c',
    'instance.param.jumpscale': True
}
vmMaster = j.atyourservice.new(name='node.ms1', instance='agent8_master', args=data)
vmMaster.install()

# get ssh client to the VM
cl = vmMaster.actions._getSSHClient(vmMaster)

# secure ssh
ubuntu = j.ssh.ubuntu.get(cl)
ubuntu.network.setHostname('agent8-master')

masterIp, _ = ubuntu.network.ipGet('eth0')
# TODO fix secureSSH method, not working now.
# unix = j.ssh.unix.get(cl)
# unix.secureSSH(sshkeypath='$(vnas_private_key)', recoverypasswd='$(vnas_recovery_password)')

cl.package_install('mercurial')

data = {
    'instance.param.disk': 0,
    'instance.param.mem': 100,
    'instance.param.passwd': '',
    'instance.param.port': 6379,
    'instance.param.unixsocket': 0
}

redis = j.atyourservice.new(name='redis', parent=vmMaster, args=data)
redis.consume('node', vmMaster.instance)
redis.install(deps=True)

data = {
    'instance.gopath': '/opt/go_workspace',
    'instance.goroot': '/opt/go'
}

redis = j.atyourservice.new(name='go', parent=vmMaster, args=data)
redis.consume('node', vmMaster.instance)
redis.install(deps=True)

# if this is not a clean machine make sure to stop agentcontroller8

cl.run('ays stop -n agentcontroller8')

# running tests.
with shell_env(GOROOT='/opt/go', GOPATH='/opt/go_workspace', PATH='PATH=$PATH:/opt/go/bin'):
    cl.run('go get -u -t -f github.com/Jumpscale/agent8')
    cl.run('go get -u -t -f github.com/Jumpscale/agentcontroller8')

    cl.run('go test -v github.com/Jumpscale/agent8/tests')


# installing controller and client from @ys
data = {
    'instance.param.redis.host': 'localhost:6379',
    'instance.param.redis.password': '',
    'instance.param.webservice.host': ':8966'
}

controller = j.atyourservice.new(name='agentcontroller8', parent=vmMaster, args=data)
controller.consume('node', vmMaster.instance)
controller.install(deps=True)

client = j.atyourservice.new(name='agentcontroller8_client', parent=vmMaster, args=data)
client.consume('node', vmMaster.instance)
client.install(deps=True)

# start the controller service
cl.run('ays start -n agentcontroller8')

# create 3 agents
# create the vn on ms1
for i in range(3):
    nid = i + 1
    data = {
        'instance.param.name': 'agent8_%s' % nid,
        'instance.param.memsize': 1,
        'instance.param.ssdsize': 10,
        'instance.param.imagename': 'ubuntu.14.04.x64',
        'instance.paran.ssh.shell': '/bin/bash -l -c',
        'instance.param.jumpscale': True
    }

    vmAgent = j.atyourservice.new(name='node.ms1', instance='agent8_%d' % nid, args=data)
    vmAgent.install()

    agentCl = vmMaster.actions._getSSHClient(vmAgent)

    ubuntu = j.ssh.ubuntu.get(agentCl)
    ubuntu.network.setHostname('agent8-%d' % nid)

    data = {
        'instance.agentcontroller': 'http://%s:8966/' % masterIp,
        'instance.gid': 1,
        'instance.nid': nid,
        'instance.roles': '',
    }

    agent = j.atyourservice.new(name='agent8', parent=vmAgent, args=data)
    agent.consume('node', vmAgent.instance)
    agent.install(deps=True)

# get ssh client to the VM
cl = vmMaster.actions._getSSHClient(vmMaster)

# test setup.
script = """
from JumpScale import j
import json

client = j.clients.ac.get(port=6379)

for i in range(3):
    nid = i + 1

    osinfo = client.get_os_info(1, nid)
    assert osinfo['hostname'] == 'agent8-%s' % nid, 'Invalid os response from agent'
"""

cl.file_write('/root/test.py', script)
cl.run('jspython /root/test.py')
