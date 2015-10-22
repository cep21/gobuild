package main
import (
	"golang.org/x/net/context"
	"os/exec"
	"io"
)

type cmdBuild struct {
	dirs []string
	cache *templateCache

	verboseLog logger
	errorLog logger

	storageDir string

	cmdStdout io.Writer
	cmdStderr io.Writer
}

func (c *cmdBuild) Run(ctx context.Context) error {
	allErrs := make([]error, 0, len(c.dirs))
	for _, dir := range c.dirs {
		stdout :=
		err := c.buildDir(ctx, dir, stdout, stderr)
		if err != nil {
			c.errorLog.Printf("Error building directory %s: %s", dir, err)
			allErrs = append(allErrs, err)
		}
	}
	return multiErr(allErrs)
}

func (c *cmdBuild) buildDir(ctx context.Context, dir string, stdout io.Writer, stderr io.Writer) error {
	c.verboseLog.Printf("Building directory %s", dir)
	tmpl, err := c.cache.loadInDir(dir)
	if err != nil {
		return wraperr(err, "cannot load cache for directory %s", dir)
	}
	buildFlags := tmpl.BuildFlags()
	cmdName := "go"
	cmdArgs := make([]string, 0, len(buildFlags) + 1)
	cmdArgs = append(cmdArgs, "build")
	cmdArgs = append(cmdArgs, buildFlags)
	cmd := exec.Command(cmdName, cmdArgs...)
	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return wraperr(err, "unable to finish running build")
	}
	return nil
}
