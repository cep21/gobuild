package main

var defaultTemplate = `
[vars]
  ignoreDirs = [".git", "Godeps", "vendor"]
  stopLoadingParent = [".git"]
  buildFlags = ["."]
  testFlags = ["-covermode", "atomic", "-race", "-timeout", "10s", "-cpu", "4", "-parallel", "8"]
  artifactsEnv = "CIRCLE_ARTIFACTS"
  testReportEnv = "CIRCLE_TEST_REPORTS"
  duplLimit = "100"
  testCoverage = 0.0

[fix]
  [fix.commands]
    gofmt = true
    goimports = false

[metalinter]
  [metalinter.vars]
    args = ["-t", "--disable-all", "--vendor", "--min-confidence=.3", "--deadline=20s"]
  [metalinter.ignored]
    unusedunderbar = "^.*:warning: _ is unused \\(deadcode\\)$"

[gotestcoverage]
  timeout = "10s"
  cpu = "4"
  parallel = 8
  race = true
  covermode = "atomic"

[install]
  [install.goget]
    gometalinter = "github.com/alecthomas/gometalinter"
    golint = "github.com/golang/lint/golint"
    go-junit-report = "github.com/jstemmer/go-junit-report"
    goimports = "golang.org/x/tools/cmd/goimports"
    gocyclo = "github.com/alecthomas/gocyclo"
    aligncheck = "github.com/opennota/check/cmd/aligncheck"
    varcheck = "github.com/opennota/check/cmd/varcheck"
    dupl = "github.com/mibk/dupl"
    deadcode = "github.com/tsenart/deadcode"
    unconvert = "github.com/mdempsky/unconvert"
    errcheck = "github.com/kisielk/errcheck"
    interfacer = "github.com/mvdan/interfacer/cmd/interfacer"
    ineffassign = "github.com/gordonklaus/ineffassign"
    structcheck = "github.com/opennota/check/cmd/structcheck"
`
