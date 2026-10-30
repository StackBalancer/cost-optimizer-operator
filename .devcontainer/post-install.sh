#!/bin/bash
set -x

curl -Lo ./kind https://kind.sigs.k8s.io/dl/latest/kind-linux-$(go env GOARCH)
chmod +x ./kind
mv ./kind /usr/local/bin/kind

curl -Lo ./k3d https://github.com/k3d-io/k3d/releases/latest/download/k3d-linux-$(go env GOARCH)
chmod +x ./k3d
mv ./k3d /usr/local/bin/k3d

curl -L -o kubebuilder https://go.kubebuilder.io/dl/latest/linux/$(go env GOARCH)
chmod +x kubebuilder
mv kubebuilder /usr/local/bin/

KUBECTL_VERSION=$(curl -L -s https://dl.k8s.io/release/stable.txt)
curl -LO "https://dl.k8s.io/release/$KUBECTL_VERSION/bin/linux/$(go env GOARCH)/kubectl"
chmod +x kubectl
mv kubectl /usr/local/bin/kubectl

kind version
k3d version

if ! k3d cluster list | grep -q cost-optimizer; then
  k3d cluster create cost-optimizer
fi

mkdir -p ~/.kube
k3d kubeconfig get cost-optimizer > ~/.kube/config

kubebuilder version
docker --version
go version
kubectl version --client

apt-get update
apt-get install -y yq
