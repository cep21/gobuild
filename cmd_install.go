package main

import (
	"os/exec"
	"strings"
	"golang.org/x/net/context"
)

type installCmd struct {
	forceReinstall bool
	verboseLog logger
	tmpl *buildTemplate
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
		return
	}
	i.verboseLog.Printf("Installing commands %s", strings.Join(toGoget, " "))
	cmd := exec.Command("go", "get", "-u", "-f", toGoget...)
	byteOut, err := cmd.CombinedOutput()
	i.verboseLog.Printf("go get output: %s", string(byteOut))
	if err != nil {
		return wraperr(err, "Unable to run go get")
	}
	return nil
}
