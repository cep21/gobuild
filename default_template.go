package main

var defaultTemplate = `
[cmd.check]
  macros = ["varcheck","ineffassign","golint","errcheck","dupl","structcheck","aligncheck","vet","vetshadow","gocyclo","deadcode", "goimports", "gofmt"]

[cmd.test]
  macros = ["test"]

[cmd.fix]
  macros = ["gofmt-fix", "goimports-fix"]
  run-next = ["check"]

[vars]
  duplthreshold = 75
  min_confidence = 0.8
  ignoreDirs = ["^vendor$", "^Godeps$", "^\\..+$", "^.git$"]
  default = "check"
  buildfileName = "gobuild.toml"
  parallelBuildCount = 16
  stop_loading_parent = ["^.git$"]
  gocyclo_over = 10

[macro.aligncheck]
  cmd="aligncheck"
  goget="github.com/opennota/check/cmd/aligncheck"
  stdout-regex="^(?:[^:]+: )?(?P<path>[^:]+):(?P<line>\\d+):(?P<col>\\d+):\\s*(?P<message>.+)$"
  args=["."]
  if-files=[".*\\.go"]

[macro.test]
  cmd="go"
  stdout-regex="^--- FAIL: .*$\\s+(?P<path>[^:]+):(?P<line>\\d+): (?P<message>.*)$"
  args=["test", "."]
  if-files=[".*\\.go"]

[macro.deadcode]
  cmd="deadcode"
  goget="github.com/remyoudompheng/go-misc/deadcode"
  stderr-regex = " (?P<path>[^:]+):(?P<line>\\d+):(?P<col>\\d+):\\s*(?P<message>.*)$"
  args=["."]
  if-files=[".*\\.go"]

[macro.dupl]
  cmd="dupl"
  goget="github.com/mibk/dupl"
  args=["-plumbing", "-threshold", "{duplthreshold}"]
  stdout-regex="^(?P<path>[^\\s][^:]+?\\.go):(?P<line>\\d+)-\\d+:\\s*(?P<message>.*)$"
  if-files=[".*\\.go"]
  append-files=true
  cross-directory=true

[macro.errcheck]
  cmd="errcheck"
  goget="github.com/kisielk/errcheck"
  args=["-abspath"]
  stdout-regex="^(?P<path>[^:]+):(?P<line>\\d+):(?P<col>\\d+)\\t(?P<message>.*)$"
  if-files=[".*\\.go"]
  message = "error return value not checked ({{ .message }})"
  append-files=true

[macro.gocyclo]
  cmd="gocyclo"
  goget="github.com/alecthomas/gocyclo"
  args=["-over", "{gocyclo_over}"]
  stdout-regex = "^(?P<cyclo>\\d+)\\s+\\S+\\s(?P<function>\\S+)\\s+(?P<path>[^:]+):(?P<line>\\d+):(\\d+)$"
  append-files=true
  message="cyclomatic complexity {{ .cyclo }} of function {{ .function }}() is high (> {{ .gocyclo_over }})"
  if-files=[".*\\.go"]

[macro.golint]
  cmd="golint"
  goget="github.com/golang/lint/golint"
  args=["-min_confidence", "{min_confidence}"]
  if-files=[".*\\.go"]
  stdout-regex = "^(?P<path>[^\\s][^:]+?\\.go):(?P<line>\\d+):(?P<col>\\d+):\\s*(?P<message>.*)$"
  append-files=true

[macro.ineffassign]
  cmd="ineffassign"
  goget="github.com/gordonklaus/ineffassign"
  args=["-n", "."]
  if-files=[".*\\.go"]
  stdout-regex = "^(?P<path>[^\\s][^:]+?\\.go):(?P<line>\\d+):(?P<col>\\d+):\\s*(?P<message>.*)$"

[macro.structcheck]
  cmd="structcheck"
  goget="github.com/opennota/check/cmd/structcheck"
  args=["."]
  stdout-regex = "^(?:[^:]+: )?(?P<path>[^:]+):(?P<line>\\d+):(?P<col>\\d+):\\s*(?P<message>.+)$"
  if-files=[".*\\.go"]
  message="unused struct field {{ .message }}"

[macro.varcheck]
  cmd="varcheck"
  goget="github.com/opennota/check/cmd/varcheck"
  args=["."]
  stdout-regex = "^(?:[^:]+: )?(?P<path>[^:]+):(?P<line>\\d+):(?P<col>\\d+):\\s*(?P<message>\\w+)$"
  if-files=[".*\\.go"]
  message = "unused global variable {{ .message }}"

[macro.vet]
  cmd="go"
  args=["tool", "vet"]
  stderr-regex = "^(?P<path>[^\\s][^:]+?\\.go):(?P<line>\\d+):\\s*(?P<message>.*)$"
  if-files=[".*\\.go"]
  append-files=true

[macro.vetshadow]
  cmd="go"
  args=["tool", "vet", "--shadow"]
  stderr-regex = "^(?P<path>[^\\s][^:]+?\\.go):(?P<line>\\d+):\\s*(?P<message>.*)$"
  if-files=[".*\\.go"]
  append-files=true

[macro.gofmt]
  cmd="gofmt"
  args=["-s", "-l"]
  stdout-regex = "^(?P<path>.*)$"
  message = "file is not gofmted"
  if-files=[".*\\.go"]
  append-files=true

[macro.goimports]
  cmd="goimports"
  args=["-l"]
  if-files=[".*\\.go"]
  stdout-regex = "^(?P<path>.*)$"
  message = "file is not goimported"
  goget="golang.org/x/tools/cmd/goimports"
  append-files=true

[macro.gofmt-fix]
  cmd="gofmt"
  args=["-s", "-w"]
  if-files=[".*\\.go"]
  append-files=true

[macro.goimports-fix]
  cmd="goimports"
  args=["-w"]
  if-files=[".*\\.go"]
  goget="golang.org/x/tools/cmd/goimports"
  append-files=true
`
