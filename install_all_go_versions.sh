#!/bin/bash
set -x
set -e
if [ -z "$CIRCLECI" ]; then
	echo "Very likely you only want to run this setup step on circleCI"
	exit 1
fi

#TODO: Move these checks into vendor?
install_go_ver() {
  [ -d /usr/local/gover/go"$1" ] || wget -O - https://storage.googleapis.com/golang/go"$1".linux-amd64.tar.gz | sudo tar -v -C /usr/local/gover/go"$1" -xzf -
}


[ -d /usr/local/go ] && sudo mv /usr/local/go /usr/local/go_backup
mkdir -p /usr/local/gover
install_go_ver 1.5.1
install_go_ver 1.4.2

ln -s /usr/local/gover/go1.5.1 /usr/local/go
