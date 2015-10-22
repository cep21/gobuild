package main
import (
	"golang.org/x/net/context"
	"os/exec"
	"path/filepath"
	"fmt"
	"os"
	"golang.org/x/tools/cover"
)

type goCoverageCheck struct {
	dirs []string
	cache *templateCache
	storageDir string
	requiredCoverage float64
	verboseLog logger
	errLog logger
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

func (g *goCoverageCheck) runForDir(dir string) error {
	template, err := g.cache.loadInDir(dir)
	if err != nil {
		return wraperr(err, "unable to load cache for %s", dir)
	}
	coverArgs := template.TestCoverageArgs()
	cmdName := "go"
	bestGuessFilename := sanitizeFilename(dir)
	coverprofile := filepath.Join(g.storageDir, fmt.Sprintf("%s.code_coverage.txt", bestGuessFilename))
	stderrFilename := filepath.Join(g.storageDir, fmt.Sprintf("%s.stderr.txt", bestGuessFilename))
	stdoutFilename := filepath.Join(g.storageDir, fmt.Sprintf("%s.stdout.txt", bestGuessFilename))

	openFiles := make([]os.File, 2)
	for i, fname := range []string{stdoutFilename, stderrFilename} {
		stream, err := os.Create(fname)
		if err != nil {
			return wraperr(err, "could not open %d filename %s", i, stderrFilename)
		}
		openFiles[i] = stream
		defer func() {
			logIfErr(stream.Close(), "cannot close %d file %s", i, stderrFilename)
		}()
	}

	coverArgs = append(coverArgs, "-coverprofile", coverprofile)

	cmd := exec.Command(cmdName, coverArgs...)
	cmd.Stdout = openFiles[0]
	cmd.Stderr = openFiles[0]
	cmd.Dir = dir
	runErr := cmd.Run()
	if runErr != nil {
		return wraperr(err, "test command failed")
	}

	coverage, err := calculateCoverage(coverprofile)
	if err != nil {
		return wraperr(err, "unable to calculate coverage")
	}
	if coverage + .001 < g.requiredCoverage {
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
