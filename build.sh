#!/bin/bash
set -e

SRCDIR=/
BUILDDIR=/build

mkdir -p ${BUILDDIR} 2>/dev/null
cd ${BUILDDIR}
echo "Cloning coredns repo..."
git clone https://github.com/coredns/coredns.git

cd coredns
git checkout v1.6.1

echo "Patching plugin config..."
ed plugin.cfg <<EOED
21a
ens:github.com/wealdtech/coredns-ens
.
w
q
EOED

# The Kubernetes dependencies result in an invalid package so replace them
echo "Patching go modules..."
cp go.mod go.mod.orig
egrep -v k8s.io/ go.mod.orig >go.mod
ed go.mod <<EOED
a
replace github.com/wealdtech/coredns-ens => ../../coredns-ens
.
52a
k8s.io/api kubernetes-1.14.1
k8s.io/apimachinery kubernetes-1.14.1
k8s.io/client-go kubernetes-1.14.1
github.com/wealdtech/coredns-ens v1.1.0
.
w
q
EOED

echo "Building..."
make SHELL='sh -x' CGO_ENABLED=1 coredns

cp coredns ${SRCDIR}
cd ${SRCDIR}
rm -r ${BUILDDIR}
