#!/bin/bash
set -ex

CIRCLEUTIL_TAG="v1.0"

function do_cache() {
  git clone https://github.com/signalfx/circleutil.git "$HOME/circleutil"
  (
    cd "$HOME/circleutil"
    git reset --hard $CIRCLEUTIL_TAG
  )
  . "$HOME/circleutil/scripts/common.sh"
  "$HOME/circleutil/scripts/install_all_go_versions.sh"
  "$HOME/circleutil/scripts/install_gobuild_lints.sh"
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

