#!/bin/bash
set -e

SRCDIR=`pwd`
BUILDDIR=`mktemp -d`

mkdir -p ${BUILDDIR} 2>/dev/null
cd ${BUILDDIR}
git clone https://github.com/coredns/coredns.git
cd coredns
git checkout v1.6.1
ed plugin.cfg <<EOED
21a
ens:github.com/wealdtech/coredns-ens
.
w
q
EOED
# The Kubernetes dependencies result in an invalid package so replace them
cp go.mod go.mod.orig
egrep -v k8s.io/ go.mod.orig >go.mod
ed go.mod <<EOED
51a
k8s.io/api kubernetes-1.14.1
k8s.io/apimachinery kubernetes-1.14.1
k8s.io/client-go kubernetes-1.14.1
github.com/wealdtech/coredns-ens v1.1.0
.
w
q
EOED
make SHELL='sh -x' CGO_ENABLED=1 coredns
cp coredns ${SRCDIR}
cd ${SRCDIR}
chmod -R 755 ${BUILDDIR}/coredns/.git/objects/pack
rm -r ${BUILDDIR}
