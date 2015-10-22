package main

import (
	"io"
	"os/exec"
	"strings"

	"github.com/cep21/gobuild/internal/golang.org/x/net/context"
)

type installCmd struct {
	forceReinstall bool
	verboseLog     logger
	errLog         logger
	tmpl           *buildTemplate

	stdoutOutput io.Writer
	stderrOutput io.Writer
}

func (i *installCmd) Run(ctx context.Context) error {
	toGoget := []string{}
	for binname, goget := range i.tmpl.Install.Goget {
		execPath, err := exec.LookPath(binname)
		if err != nil || i.forceReinstall {
			toGoget = append(toGoget, goget)
			i.verboseLog.Printf("Need to install %s => %s", binname, goget)
		} else {
			i.verboseLog.Printf("Found %s at %s", binname, execPath)
		}
	}

	if len(toGoget) == 0 {
		i.verboseLog.Printf("No commands to install")
		return nil
	}
	i.verboseLog.Printf("Installing commands %s", strings.Join(toGoget, " "))
	cmd := exec.Command("go", "get", "-u", "-f")
	cmd.Args = append(cmd.Args, toGoget...)

	cmd.Stdout = i.stdoutOutput
	cmd.Stderr = i.stderrOutput
	if err := cmd.Run(); err != nil {
		return wraperr(err, "Unable to run go get")
	}
	return nil
}
