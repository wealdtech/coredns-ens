#!/bin/bash

onexit() {
  cd ${SRCDIR}
  chmod -R 755 ${BUILDDIR}/coredns/.git/objects/pack 2>/dev/null
  rm -r ${BUILDDIR} 2>/dev/null
  exit 1
}
trap onexit SIGHUP SIGINT SIGTERM

DOCKERGROUP=$1

SRCDIR=`pwd`
BUILDDIR=`mktemp -d`

mkdir -p ${BUILDDIR} 2>/dev/null
cd ${BUILDDIR}
git clone git@github.com:coredns/coredns
cd coredns
git checkout v1.6.1
ed plugin.cfg <<EOED
21a
ens:github.com/wealdtech/coredns-ens
.
w
q
EOED
sed -i -e 's/CGO_ENABLED:=0/CGO_ENABLED:=1/' Makefile
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
make
cp coredns ${SRCDIR}
cd ${SRCDIR}
chmod -R 755 ${BUILDDIR}/coredns/.git/objects/pack
rm -r ${BUILDDIR}

if [ ! -z "$DOCKERGROUP" ] ; then
  echo "Creating docker image ${DOCKERGROUP}/coredns"
  docker build -t ${DOCKERGROUP}/coredns .
fi
