#!/bin/bash
set -e

SRCDIR=/
BUILDDIR=/build

mkdir -p ${BUILDDIR} 2>/dev/null
cd ${BUILDDIR}
echo "Cloning coredns repo..."
git clone https://github.com/coredns/coredns.git

cd coredns
git checkout v1.8.3

echo "Patching plugin config..."
ed plugin.cfg <<EOED
/rewrite:rewrite
a
ens:github.com/wealdtech/coredns-ens
.
w
q
EOED

# Add our module to coredns.
echo "Patching go modules..."
ed go.mod <<EOED
a
replace github.com/wealdtech/coredns-ens => ../../coredns-ens
.
/^)
-1
a
	github.com/wealdtech/coredns-ens v1.3.1
.
w
q
EOED

go get github.com/wealdtech/coredns-ens@v1.3.1
go get
go mod download

echo "Building..."
make SHELL='sh -x' CGO_ENABLED=1 coredns

cp coredns ${SRCDIR}
cd ${SRCDIR}
rm -r ${BUILDDIR}
