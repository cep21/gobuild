package main

var defaultTemplate = `
[vars]
  ignoreDirs = [".git", "Godeps", "vendor"]
  stopLoadingParent = [".git"]

[install]
  [install.goget]
    gometalinter = "github.com/alecthomas/gometalinter"
    golint = "github.com/golang/lint/golint"
    goimports = "golang.org/x/tools/cmd/goimports"
    gocyclo = "github.com/fzipp/gocyclo"
    aligncheck = "github.com/opennota/check/cmd/aligncheck"
    varcheck = "github.com/opennota/check/cmd/varcheck"
    dupl = "github.com/mibk/dupl"
`
