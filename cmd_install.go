package main

import (
	"io"
	"os/exec"
	"strings"

	"golang.org/x/net/context"
)

type installCmd struct {
	forceReinstall bool
	verboseLog     logger
	errLog         logger
	tmpl           *buildTemplate

	stdoutOutput cmdOutputStreamer
	stderrOutput cmdOutputStreamer
}

func (i *installCmd) Run(ctx context.Context) error {
	toGoget := []string{}
	for binname, goget := range i.tmpl.Install.Goget {
		_, err := exec.LookPath(binname)
		if err != nil || i.forceReinstall {
			toGoget = append(toGoget, goget)
		}
	}

	if len(toGoget) == 0 {
		i.verboseLog.Printf("No commands to install")
		return nil
	}
	i.verboseLog.Printf("Installing commands %s", strings.Join(toGoget, " "))
	cmd := exec.Command("go", "get", "-u", "-f")
	cmd.Args = append(cmd.Args, toGoget...)
	stdout, err := i.stdoutOutput.GetCmdOutput("")
	if err != nil {
		return wraperr(err, "cannot get stdout file")
	}
	stderr, err := i.stderrOutput.GetCmdOutput("")
	if err != nil {
		return wraperr(err, "cannot get stderr file")
	}

	for _, s := range []io.Closer{stdout, stderr} {
		defer func() {
			logIfErr(s.Close(), i.errLog, "could not flush install output file")
		}()
	}

	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return wraperr(err, "Unable to run go get")
	}
	return nil
}
