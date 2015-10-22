package main
import (
"golang.org/x/net/context"
"strings"
	"os/exec"
)

type fixCmd struct {
	dirs []string
	chunkSize int

	verboseOut logger
	errOut logger
}

func (f *fixCmd) Run(ctx context.Context) error {
	goFiles, err := filesWithGlobInDir(f.dirs, "*.go")
	if err != nil {
		return wraperr(err, "dupl *.go glob search failed for %s", strings.Join(f.dirs, ", "))
	}

	if err := f.fmtCmd("gofmt", []string{"-s", "-w"}, goFiles); err != nil {
		return wraperr(err, "cannot gofmt correctly")
	}
	if err := f.fmtCmd("goimports", []string{"-w"}, goFiles); err != nil {
		return wraperr(err, "cannot goimports correctly")
	}
	return nil
}

func (f *fixCmd) fmtCmd(cmdName string, args []string, goFiles []string) error {
	for _, chunk := range chunkStrings(goFiles, f.chunkSize) {
		f.verboseOut.Printf("running %s on %s", cmdName, strings.Join(chunk, ", "))
		cmd := exec.Command(cmdName, append(args, chunk)...)
		bout, err := cmd.CombinedOutput()
		if err != nil {
			return wraperr(err, "unable to run %s correctly", cmdName)
		}
		if len(bout) > 0 {
			f.errOut.Printf("Unexpected %s output: %s", cmdName, string(bout))
			return wraperr(err, "Unexpected %s output", cmdName)
		}
	}
	return nil
}

func chunkStrings(strs []string, size int) [][]string {
	ret := make([][]string, 0, len(strs)/size + 1)
	cur := make([]string, 0, size)
	for _, str := range strs {
		cur = append(cur, str)
		if len(cur) == size {
			ret = append(ret, cur)
			cur = make([]string, 0, size)
		}
	}
	if len(cur) > 0 {
		ret = append(ret, cur)
	}
	return ret
}
