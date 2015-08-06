from JumpScale import j

from fabric.context_managers import shell_env

# create the vn on ms1
data = {
    'instance.param.name': 'superagent_master',
    'instance.param.memsize': 1,
    'instance.param.ssdsize': 10,
    'instance.param.imagename': 'ubuntu.14.04.x64',
    'instance.paran.ssh.shell': '/bin/bash -l -c',
    'instance.param.jumpscale': True
}
vmMaster = j.atyourservice.new(name='node.ms1', instance='superagent_master', args=data)
vmMaster.install()

# get ssh client to the VM
cl = vmMaster.actions._getSSHClient(vmMaster)

# secure ssh
ubuntu = j.ssh.ubuntu.get(cl)
ubuntu.network.setHostname('superagent-master')

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

# if this is not a clean machine make sure to stop jsagentcontroller_go

cl.run('ays stop -n jsagentcontroller_go')

# running tests.
with shell_env(GOROOT='/opt/go', GOPATH='/opt/go_workspace', PATH='PATH=$PATH:/opt/go/bin'):
    cl.run('go get -u -t -f github.com/Jumpscale/jsagent')
    cl.run('go get -u -t -f github.com/Jumpscale/jsagentcontroller')

    cl.run('go test -v github.com/Jumpscale/jsagent/tests')


# installing controller and client from @ys
data = {
    'instance.param.redis.host': 'localhost:6379',
    'instance.param.redis.password': '',
    'instance.param.webservice.host': ':8966'
}

controller = j.atyourservice.new(name='jsagentcontroller_go', parent=vmMaster, args=data)
controller.consume('node', vmMaster.instance)
controller.install(deps=True)

client = j.atyourservice.new(name='jsagentcontroller_go_client', parent=vmMaster, args=data)
client.consume('node', vmMaster.instance)
client.install(deps=True)

# start the controller service
cl.run('ays start -n jsagentcontroller_go')

# create 3 agents
# create the vn on ms1
for i in range(3):
    nid = i + 1
    data = {
        'instance.param.name': 'superagent_%s' % nid,
        'instance.param.memsize': 1,
        'instance.param.ssdsize': 10,
        'instance.param.imagename': 'ubuntu.14.04.x64',
        'instance.paran.ssh.shell': '/bin/bash -l -c',
        'instance.param.jumpscale': True
    }

    vmAgent = j.atyourservice.new(name='node.ms1', instance='superagent_%d' % nid, args=data)
    vmAgent.install()

    agentCl = vmMaster.actions._getSSHClient(vmAgent)

    ubuntu = j.ssh.ubuntu.get(agentCl)
    ubuntu.network.setHostname('superagent-%d' % nid)

    data = {
        'instance.agentcontroller': 'http://%s:8966/' % masterIp,
        'instance.gid': 1,
        'instance.nid': nid
    }

    agent = j.atyourservice.new(name='superagent', parent=vmAgent, args=data)
    agent.consume('node', vmAgent.instance)
    agent.install(deps=True)
