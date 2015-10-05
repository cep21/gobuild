package main

var defaultTemplate = `
[cmd.check]
  macros = ["varcheck","defercheck","ineffassign","golint","errcheck","dupl","structcheck","aligncheck","vet","vetshadow","gocyclo","deadcode", "gofmt", "goimports"]

[cmd.fix]
  macros = ["gofmt-fix", "goimports-fix"]
  run-next = ["check"]

[vars]
  duplthreshold = 75
  min_confidence = 0.8
  ignoreDirs = ["vendor", "Godeps", "^\..*$"]
  default = "fix"
  buildfileName = "gobuild.toml"
  stop_loading_parent = [".git"]

[macro.aligncheck]
  cmd="aligncheck"
  goget="github.com/opennota/check/cmd/aligncheck"
  args=["{path}"]
  if-files=["*.go"]

[macro.deadcode]
  cmd="deadcode"
  goget="github.com/remyoudompheng/go-misc/deadcode"
  args=["{path}"]
  if-files=["*.go"]

[macro.dupl]
  cmd="dupl"
  goget="github.com/mibk/dupl"
  args=["-plumbing", "-threshold", "{duplthreshold}", "."]
  only-at-root=true

[macro.errcheck]
  cmd="errcheck"
  goget="github.com/kisielk/errcheck"
  args=["{path}"]
  if-files=["*.go"]

[macro.gocyclo]
  cmd="gocyclo"
  goget="github.com/alecthomas/gocyclo"
  args=["{path}"]
  if-files=["*.go"]

[macro.golint]
  cmd="golint"
  goget="github.com/golang/lint/golint"
  args=["-min_confidence", "{min_confidence}", "{path}"]
  if-files=["*.go"]

[macro.ineffassign]
  cmd="ineffassign"
  goget="github.com/gordonklaus/ineffassign"
  args=["-n", "."]
  if-files=["*.go"]

[macro.structcheck]
  cmd="structcheck"
  goget="github.com/opennota/check/cmd/structcheck"
  args=["{path}"]
  if-files=["*.go"]

[macro.varcheck]
  cmd="varcheck"
  goget="github.com/opennota/check/cmd/varcheck"
  args=["{path}"]
  if-files=["*.go"]

[macro.vet]
  cmd="go"
  args=["tool", "vet", "{path-go}"]
  if-files=["*.go"]

[macro.vetshadow]
  cmd="go"
  args=["tool", "vet", "--shadow", "{path-go}"]
  if-files=["*.go"]

[macro.gofmt]
  cmd="gofmt"
  args=["-s", "-l", "{path-go}"]
  if-files=["*.go"]

[macro.goimports]
  cmd="goimports"
  args=["-l", "{path-go}"]
  if-files=["*.go"]
  goget="golang.org/x/tools/cmd/goimports"

[macro.gofmt-fix]
  cmd="gofmt"
  args=["-s", "-w", "{path-go}"]
  if-files=["*.go"]

[macro.goimports-fix]
  cmd="goimports"
  goget="golang.org/x/tools/cmd/goimports"
  args=["-w", "{path-go}"]
  if-files=["*.go"]
`
