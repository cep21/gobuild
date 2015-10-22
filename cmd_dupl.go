package main

import (
	"bytes"
	"io"
	"os/exec"
	"strings"

	"github.com/cep21/gobuild/internal/golang.org/x/net/context"
)

type duplCmd struct {
	verboseLog logger
	htmlOut    io.Writer
	consoleOut io.Writer
	tmpl       *buildTemplate
	dirs       []string
}

func (d *duplCmd) Run(ctx context.Context) error {
	d.verboseLog.Printf("Running dupl -plumbing command")
	regDuplOut, err := d.runDupl(ctx, []string{"-plumbing"})
	if err != nil {
		d.verboseLog.Printf("Regular dupl output: %s", string(regDuplOut))
		return wraperr(err, "unable to correctly run regular dupl")
	}

	d.verboseLog.Printf("Running dupl -html command")
	htmlDuplOut, err := d.runDupl(ctx, []string{"-html"})
	if err != nil {
		d.verboseLog.Printf("html dupl output: %s", string(htmlDuplOut))
		return wraperr(err, "unable to correctly run html dupl")
	}
	n, err := d.consoleOut.Write(regDuplOut)
	if err != nil {
		return wraperr(err, "could not copy dupl output, wrote %d", n)
	}
	n, err = d.htmlOut.Write(htmlDuplOut)
	if err != nil {
		return wraperr(err, "could not copy html dupl output, wrote %d", n)
	}
	return nil
}

func (d *duplCmd) runDupl(ctx context.Context, extraArgs []string) ([]byte, error) {
	cmdName := "dupl"
	args := append(d.tmpl.DuplArgs(), extraArgs...)
	goFiles, err := filesWithGlobInDir(d.dirs, "*.go")
	if err != nil {
		return nil, wraperr(err, "dupl *.go glob search failed for %s", strings.Join(d.dirs, ", "))
	}
	buf := bytes.Buffer{}
	_, err = io.Copy(&buf, bytes.NewBufferString(strings.Join(goFiles, "\n")))
	if err != nil {
		return nil, wraperr(err, "could not copy gofiles into buffer")
	}
	cmd := exec.Command(cmdName, args...)
	cmd.Stdin = &buf
	return cmd.CombinedOutput()
}
