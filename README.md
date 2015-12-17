# gobuild [![Circle CI](https://circleci.com/gh/cep21/gobuild.svg?style=svg)](https://circleci.com/gh/cep21/gobuild)

Code lint and build tool for go code on build servers and developer machines.

## Features

* Runs in windows/linux/mac without extra dependencies
* Native support for [CircleCI](https://circleci.com/)
* Simple usage with minimal command line parameters
* Can run in subdirectories with understood usage of operating on sub-subdirectories
* Auto download of dependencies
* Easy to read config format
* Can auto format code for developers
* Easy to understand error messages
* Optional verbose output
* Inheritance of usage via directory structure
* Understands limiting directory search by git project
* Can ignore directories, files, or messages
* Easy to templatize

## Expected usage

### as a developer

#### to check everything

```
gobuild
```

#### to just run lints

```
gobuild lint
```

#### to auto format code

```
gobuild fix
```

### as a build system

```
gobuild -verbose -verbosefile build_output.txt
```

## Configuration

Configuration options are loaded from a `gobuild.toml` file in the root of the project and merged with the default configuration.

### Default configuration
```toml
[vars]
  ignoreDirs = [".git", "Godeps", "vendor"]
  stopLoadingParent = [".git"]
  buildFlags = ["."]
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
```

## Why not just use

### go build

I want to automate running of gofmt, go vet, and other checks.  There are also
multiple binaries that can be built.

### bash scripts

I want easy windows support without installing cygwin and other windows
dependencies.  I also want to templatize the common parts.  Finally, I
subjectively think go is a better cross platform language than bash.

### gometalinter

I want subdirectory reconfiguration and auto formatting.  Also, the simplicity
of a running the command without any arguments and it doing the right thing was
attractive.

### cmake

Cmake on windows requires a bit more setup than I want.  Also sharing templates
was more work than I wanted to push on users

### maven

Configuration format (xml) is a bit difficult to read.
