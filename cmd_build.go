package main

import (
	"io"
	"os/exec"

	"github.com/cep21/gobuild/internal/golang.org/x/net/context"
)

type cmdBuild struct {
	dirs  []string
	cache *templateCache

	verboseLog logger
	errorLog   logger

	cmdStdout cmdOutputStreamer
	cmdStderr cmdOutputStreamer
}

func (c *cmdBuild) Run(ctx context.Context) error {
	allErrs := make([]error, 0, len(c.dirs))
	for _, dir := range c.dirs {
		err := c.buildDir(ctx, dir)
		if err != nil {
			c.errorLog.Printf("Error building directory %s: %s", dir, err)
			allErrs = append(allErrs, err)
		}
	}
	return multiErr(allErrs)
}

func (c *cmdBuild) buildDir(ctx context.Context, dir string) error {
	stdout, err := c.cmdStdout.GetCmdOutput(dir)
	if err != nil {
		return wraperr(err, "cannot create stdout output for %s", dir)
	}

	stderr, err := c.cmdStderr.GetCmdOutput(dir)
	if err != nil {
		return wraperr(err, "cannot create stderr output for %s", dir)
	}

	for _, s := range []io.Closer{stdout, stderr} {
		defer func(s io.Closer) {
			logIfErr(s.Close(), c.errorLog, "could not flush build output file")
		}(s)
	}

	c.verboseLog.Printf("Building directory %s", dir)
	tmpl, err := c.cache.loadInDir(dir)
	if err != nil {
		return wraperr(err, "cannot load cache for directory %s", dir)
	}
	buildFlags := tmpl.BuildFlags()
	cmdName := "go"
	cmdArgs := make([]string, 0, len(buildFlags)+1)
	cmdArgs = append(cmdArgs, "build")
	cmdArgs = append(cmdArgs, buildFlags...)
	cmd := exec.Command(cmdName, cmdArgs...)
	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return wraperr(err, "unable to finish running build for %s", dir)
	}
	return nil
}
