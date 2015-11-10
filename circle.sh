#!/bin/bash
set -ex

export GOPATH_INTO="$HOME/installed_gotools"

CIRCLEUTIL_TAG="v1.7"
export GOLANG_VERSION="1.5.1"
export GO15VENDOREXPERIMENT="1"
export GOROOT="$HOME/go_circle"
export GOPATH="$HOME/.go_circle"
export PATH="$GOROOT/bin:$GOPATH/bin:$GOPATH_INTO:$PATH"
export IMPORT_PATH="github.com/cep21/gobuild"
export CIRCLE_ARTIFACTS="${CIRCLE_ARTIFACTS-/tmp}"

function do_cache() {
  [ ! -d "$HOME/circleutil" ] && git clone https://github.com/signalfx/circleutil.git "$HOME/circleutil"
  (
    cd "$HOME/circleutil"
    git fetch -a -v
    git fetch --tags
    git reset --hard $CIRCLEUTIL_TAG
  )
  . "$HOME/circleutil/scripts/common.sh"
  . "$HOME/circleutil/scripts/install_all_go_versions.sh"
  GOPATH_INTO=$GOPATH_INTO . "$HOME/circleutil/scripts/versioned_goget.sh" "github.com/cep21/gobuild:v1.0"
}

function do_test() {
  . "$HOME/circleutil/scripts/common.sh"
  SRC_PATH="$GOPATH/src/github.com/cep21/gobuild"
  copy_local_to_path "$SRC_PATH"
  (
    cd "$SRC_PATH"
    go test .
    go build . && ./gobuild -verbose -verbosefile "$CIRCLE_ARTIFACTS/gobuildout.txt"
  )
}

function do_all() {
  do_cache
  do_test
}

case "$1" in
  cache)
    do_cache
    ;;
  test)
    do_test
    ;;
  all)
    do_all
    ;;
  *)
  echo "Usage: $0 {cache|test|all}"
    exit 1
    ;;
esac

