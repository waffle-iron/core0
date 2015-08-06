from JumpScale import j

from fabric.context_managers import shell_env

# create the vn on ms1
data = {
    'instance.param.name': 'superagent_test',
    'instance.param.memsize': 1,
    'instance.param.ssdsize': 10,
    'instance.param.imagename': 'ubuntu.14.04.x64',
    'instance.paran.ssh.shell': '/bin/bash -l -c',
    'instance.param.jumpscale': True
}
vmMaster = j.atyourservice.new(name='node.ms1', instance='superagent_test', args=data)
vmMaster.install()

# get ssh client to the VM
cl = vmMaster.actions._getSSHClient(vmMaster)

# secure ssh
ubuntu = j.ssh.ubuntu.get(cl)
ubuntu.network.setHostname('superagent-test')
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


with shell_env(GOROOT='/opt/go', GOPATH='/opt/go_workspace', PATH='PATH=$PATH:/opt/go/bin'):
    cl.run('go get -u -t -f github.com/Jumpscale/jsagent')
    cl.run('go get -u -t -f github.com/Jumpscale/jsagentcontroller')

    cl.run('go test -v github.com/Jumpscale/jsagent/tests')
