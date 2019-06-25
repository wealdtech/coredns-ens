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
echo ens:github.com/wealdtech/coredns-ens >>plugin.cfg
sed -i -e 's/CGO_ENABLED:=0/CGO_ENABLED:=1/' Makefile
make
cp coredns ${SRCDIR}
cd ${SRCDIR}
chmod -R 755 ${BUILDDIR}/coredns/.git/objects/pack
rm -r ${BUILDDIR}

if [ ! -z "$DOCKERGROUP" ] ; then
  echo "Creating docker image ${DOCKERGROUP}/coredns"
  docker build -t ${DOCKERGROUP}/coredns .
fi
