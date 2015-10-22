package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"golang.org/x/net/context"
	"golang.org/x/tools/cover"
"strings"
)

type goCoverageCheck struct {
	dirs               []string
	cache              *templateCache
	coverProfileOutTo  cmdOutputStreamer
	testStdoutOutputTo cmdOutputStreamer
	testStderrOutputTo cmdOutputStreamer
	requiredCoverage   float64
	verboseLog         logger
	errLog             logger
}

func (g *goCoverageCheck) Run(ctx context.Context) error {
	allErrs := make([]error, 0, len(g.dirs))
	for _, d := range g.dirs {
		g.verboseLog.Printf("Running test %s", d)
		if err := g.runForDir(d); err != nil {
			g.errLog.Printf("Test failure on %s: %s", d, err.Error())
			allErrs = append(allErrs, err)
		}
	}
	return multiErr(allErrs)
}

type hasName interface {
	Name() string
}

var _ hasName = &os.File{}

func (g *goCoverageCheck) runForDir(dir string) error {
	template, err := g.cache.loadInDir(dir)
	if err != nil {
		return wraperr(err, "unable to load cache for %s", dir)
	}
	coverArgs := append([]string{"test"}, template.TestCoverageArgs()...)
	cmdName := "go"
	coverprofile, err := g.coverProfileOutTo.GetCmdOutput(dir)
	if err != nil {
		return wraperr(err, "coverprofile generation failed for %s", dir)
	}
	stdout, err := g.testStdoutOutputTo.GetCmdOutput(dir)
	if err != nil {
		return wraperr(err, "stdout generation failed for %s", dir)
	}
	stderr, err := g.testStderrOutputTo.GetCmdOutput(dir)
	if err != nil {
		return wraperr(err, "stderr generation failed for %s", dir)
	}
	for _, s := range []io.Closer{stdout, stderr} {
		defer func(s io.Closer) {
			logIfErr(s.Close(), g.errLog, "could not flush test output file")
		}(s)
	}

	// Note: this panics if the coverprofile doesn't return File types
	coverprofileName := coverprofile.(hasName).Name()
	coverArgs = append(coverArgs, "-coverprofile", coverprofileName, ".")

	if err := coverprofile.Close(); err != nil {
		return wraperr(err, "unable to generate coverprofile file")
	}

	cmd := exec.Command(cmdName, coverArgs...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Dir = dir
	g.verboseLog.Printf("Running [cmd=%s args=%s dir=%s]", cmd.Path, strings.Join(cmd.Args, " "), cmd.Dir)
	err = cmd.Run()
	if err != nil {
		return wraperr(err, "test command failed")
	}

	coverage, err := calculateCoverage(coverprofileName)
	if err != nil {
		return wraperr(err, "unable to calculate coverage")
	}
	if coverage+.001 < g.requiredCoverage {
		return fmt.Errorf("code coverage %f < required %f for %s", coverage, g.requiredCoverage, dir)
	}
	return nil
}

func calculateCoverage(coverprofile string) (float64, error) {
	profiles, err := cover.ParseProfiles(coverprofile)
	if err != nil {
		if os.IsNotExist(err) {
			return 0.0, nil
		}
		return 0.0, wraperr(err, "cannot parse coverage profile file %s", coverprofile)
	}
	total := 0
	covered := 0
	for _, profile := range profiles {
		for _, block := range profile.Blocks {
			total += block.NumStmt
			if block.Count > 0 {
				covered += block.NumStmt
			}
		}
	}
	if total == 0 {
		return 0.0, nil
	}
	return float64(covered) / float64(total) * 100, nil
}
