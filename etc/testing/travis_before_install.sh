#!/bin/bash

set -ex

echo 'DOCKER_OPTS="-H unix:///var/run/docker.sock -s devicemapper"' | tee /etc/default/docker > /dev/null

KIND_CACHE_PATH="~/cached-deps/kind-$KIND_VERSION"

# Install jq and ag
sudo apt-get update -y
sudo apt-get install jq silversearcher-ag

# Install fuse
apt-get install -qq pkg-config fuse
modprobe fuse
chmod 666 /dev/fuse
cp etc/build/fuse.conf /etc/fuse.conf
chown root:$USER /etc/fuse.conf

# Install kubectl
# To get the latest kubectl version:
# curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt
if [ ! -f ~/cached-deps/kubectl ] ; then
    curl -L -o kubectl https://storage.googleapis.com/kubernetes-release/release/1.15.3/bin/linux/amd64/kubectl && \
        chmod +x ./kubectl && \
        mv ./kubectl ~/cached-deps/kubectl
fi

# Install KinD (Kubernetes in Docker)
if [ ! -f "${KIND_CACHE_PATH}" ]; then
    echo '>>> Downloading Kind'
    curl -sL "https://github.com/kubernetes-sigs/kind/releases/download/v0.5.1/kind-linux-amd64" -o kind && \
    chmod +x ./kind &&
    mv ./kind ~/cached-deps/kind
fi

# Install minikube
# To get the latest minikube version:
# curl https://api.github.com/repos/kubernetes/minikube/releases | jq -r .[].tag_name | sort | tail -n1
if [ ! -f ~/cached-deps/minikube ] ; then
    MINIKUBE_VERSION=v0.31.0
    curl -L -o minikube https://storage.googleapis.com/minikube/releases/${MINIKUBE_VERSION}/minikube-linux-amd64 && \
        chmod +x ./minikube && \
        mv ./minikube ~/cached-deps/minikube
fi

# Install vault
if [ ! -f ~/cached-deps/vault ] ; then
    curl -Lo vault.zip https://releases.hashicorp.com/vault/1.2.3/vault_1.2.3_linux_amd64.zip && \
        unzip vault.zip && \
        mv ./vault ~/cached-deps/vault
fi

# Install etcdctl
# To get the latest etcd version:
# curl -Ls https://api.github.com/repos/etcd-io/etcd/releases | jq -r .[].tag_name
if [ ! -f ~/cached-deps/etcdctl ] ; then
    ETCD_VERSION=v3.3.12
    curl -L https://storage.googleapis.com/etcd/${ETCD_VERSION}/etcd-${ETCD_VERSION}-linux-amd64.tar.gz | \
        tar xzf - --strip-components=1 && \
        mv ./etcdctl ~/cached-deps/etcdctl
fi
