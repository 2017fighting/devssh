# DevSSH

devssh is a tool that can help you begin remote develop on k8s.

## Features

- ssh to pod that running in k8s
- agent local git-credentials to remote

## How to

Once you make `devssh ssh`, devssh will use [k8s `client-go`](https://github.com/kubernetes/client-go) to make a `exec ssh-server` on remote pod.
Then devssh will make a ssh client by the `exec stream`, and run other commands(git-credentials agent).

## Requirement

- a service (which is your dev container) running on your k8s
- ensure devssh is installed on `/usr/local/bin/devssh` in your that service([suggest](https://github.com/devcontainers-contrib/nanolayer):`nanolayer install gh-release '2017fighting/devssh' devssh`)
- install devssh on your local machine(`go install github.com/2017fighting/devssh@latest`)

## Usage

### 1. use as cli command

`devssh ssh --svc {service} --ns {namespace} --user {user}`

### 2. use as ssh `ProxyCommand`

edit your "~/.ssh/config", and insert this

```cfg
Host dev
  ForwardAgent yes
  LogLevel error
  StrictHostKeyChecking no
  UserKnownHostsFile /dev/null
  HostKeyAlgorithms rsa-sha2-256,rsa-sha2-512,ssh-rsa
  User root
  ProxyCommand "/Users/{user}/go/bin/devssh" ssh --svc {service} --ns {namespace} --user {user}`
```

then you can use `ssh dev` to ssh into your dev container.
