#!/bin/bash

set -ex

wget -N http://www.dest-unreach.org/socat/download/socat-1.7.3.2.tar.gz
echo "ce3efc17e3e544876ebce7cd6c85b3c279fda057b2857fcaaf67b9ab8bdaf034  socat-1.7.3.2.tar.gz" | sha256sum -c -
tar zxf socat-1.7.3.2.tar.gz

cd socat-1.7.3.2

# We disable openssl so we don't need it to be installed, but we also disable some other things that should not be needed and expand the security surface area
CFLAGS=-static LDFLAGS=-static CPPFLAGS=-static CFLAGS_APPEND=-static  LDFLAGS_APPEND=-static CPPFLAGS_APPEND=-static ./configure --disable-openssl --disable-readline --disable-termios --disable-exec --disable-system --disable-ext2
make socat

cd ..

mv socat-1.7.3.2/socat $1

exit 0
