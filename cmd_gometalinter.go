package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"

	"golang.org/x/net/context"
)

type gometalinterCmd struct {
	verboseLog logger
	errLog     logger
	metaOutput cmdOutputStreamer
	dirsToLint []string
	cache      *templateCache
}

func (l *gometalinterCmd) Run(ctx context.Context) error {
	allFailures := make([]string, 0, len(l.dirsToLint))
	for _, dir := range l.dirsToLint {
		tmpl, err := l.cache.loadInDir(dir)
		if err != nil {
			return wraperr(err, "unable to load template for %s", dir)
		}
		failedLines, err := l.lintInDir(dir, tmpl)
		if err != nil {
			return wraperr(err, "unable to parse gometalinter lines")
		}
		dataParts := make([]string, 0, len(failedLines))
		for _, line := range failedLines {
			errStr := fmt.Sprintf("%s:%s", dir, line)
			dataParts = append(dataParts, errStr)
		}
		allFailures = append(allFailures, dataParts...)
		dst, err := l.metaOutput.GetCmdOutput(dir)
		if err != nil {
			return wraperr(err, "unable to output gometalinter stderr/out to file")
		}
		data := strings.Join(dataParts, "\n")
		l.verboseLog.Printf("Output metalint results to %s", dst)
		if _, err := io.WriteString(dst, data); err != nil {
			return wraperr(err, "unable to output gometalinter stderr/out to file")
		}
		if err := dst.Close(); err != nil {
			l.errLog.Printf("Unable to close output destination: %s", err.Error())
		}
	}
	return fmt.Errorf("%s", strings.Join(allFailures, "\n"))
}

var validFilenames = regexp.MustCompile("[^A-Za-z0-9\\._-]+")

func sanitizeFilename(s string) string {
	s = strings.TrimSpace(s)
	s = validFilenames.ReplaceAllString(s, "_")

	return s
}

type lintErr struct {
	errLines []string
}

func (l *lintErr) Error() string {
	p := l.errLines
	if len(p) > 10 {
		p = append(p[0:3], "...")
	}
	return strings.Join(p, "|")
}

func parseRegexes(reg []string) ([]*regexp.Regexp, error) {
	ret := make([]*regexp.Regexp, 0, len(reg))
	for _, g := range reg {
		r, err := regexp.Compile(g)
		if err != nil {
			return nil, wraperr(err, "regex won't compile: %s", g)
		}
		ret = append(ret, r)
	}
	return ret, nil
}

func matchesAny(toMatch []byte, reg []*regexp.Regexp) bool {
	for _, r := range reg {
		if r.Match(toMatch) {
			return true
		}
	}
	return false
}

func (l *gometalinterCmd) lintInDir(dir string, tmpl *buildTemplate) ([]string, error) {
	cmd := exec.Command("gometalinter")
	cmd.Dir = dir
	cmd.Args = tmpl.MetalintArgs()
	out, err := cmd.CombinedOutput()
	if err != nil {
		l.verboseLog.Printf("Error running metalinter.  We usually ignore errors anyways: %s %s", err.Error(), string(out))
	}
	l.verboseLog.Printf("Output of metalinter on %s: %s", dir, string(out))
	outToIgnore := tmpl.MetalintIgnoreLines()
	regs, err := parseRegexes(outToIgnore)
	if err != nil {
		return nil, wraperr(err, "was unable to parse regex output in dir %s", dir)
	}
	linesParse := bufio.NewScanner(bytes.NewBuffer(out))
	failedLineParses := make([]string, 0, 10)
	for linesParse.Scan() {
		line := linesParse.Bytes()
		if !matchesAny(line, regs) {
			failedLineParses = append(failedLineParses, string(line))
		}
	}
	return failedLineParses, nil
}
