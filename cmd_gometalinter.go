package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"

	"errors"

	"github.com/cep21/gobuild/internal/golang.org/x/net/context"
)

type gometalinterCmd struct {
	verboseLog logger
	errLog     logger
	metaOutput cmdOutputStreamer
	dirsToLint []string
	cache      *templateCache

	regexParseCache map[string]*regexp.Regexp
}

var errLintFailures = errors.New("gometalinter failures found")

func (l *gometalinterCmd) Run(ctx context.Context) error {
	if l.regexParseCache == nil {
		l.regexParseCache = make(map[string]*regexp.Regexp, 10)
	}
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
			errStr := fmt.Sprintf("%s/%s", dir, line)
			dataParts = append(dataParts, errStr)
		}
		allFailures = append(allFailures, dataParts...)
		dst, err := l.metaOutput.GetCmdOutput(dir)
		if err != nil {
			return wraperr(err, "unable to output gometalinter stderr/out to file")
		}
		data := strings.Join(dataParts, "\n") + "\n"
		if len(dataParts) == 0 {
			data = ""
		}
		l.verboseLog.Printf("Output metalint results to %s", dst)
		if _, err := io.WriteString(dst, data); err != nil {
			return wraperr(err, "unable to output gometalinter stderr/out to file")
		}
		if err := dst.Close(); err != nil {
			l.errLog.Printf("Unable to close output destination: %s", err.Error())
		}
	}
	if len(allFailures) == 0 {
		return nil
	}
	return errLintFailures
}

var validFilenames = regexp.MustCompile("[^A-Za-z0-9\\._-]+")

func sanitizeFilename(s string) string {
	s = strings.TrimSpace(s)
	s = validFilenames.ReplaceAllString(s, "_")

	return s
}

func (l *gometalinterCmd) parseRegexes(reg []string) ([]*regexp.Regexp, error) {
	ret := make([]*regexp.Regexp, 0, len(reg))
	for _, g := range reg {
		if r, exists := l.regexParseCache[g]; exists {
			ret = append(ret, r)
			continue
		}
		r, err := regexp.Compile(g)
		if err != nil {
			return nil, wraperr(err, "regex won't compile: %s", g)
		}
		ret = append(ret, r)
		l.regexParseCache[g] = r
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
	l.verboseLog.Printf("Running command %v", cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		l.verboseLog.Printf("Error running metalinter.  We usually ignore errors anyways: %s %s", err.Error(), string(out))
	}
	l.verboseLog.Printf("Output of metalinter on %s: %s", dir, string(out))
	outToIgnore := tmpl.MetalintIgnoreLines()
	regs, err := l.parseRegexes(outToIgnore)
	if err != nil {
		return nil, wraperr(err, "was unable to parse regex output in dir %s", dir)
	}
	l.verboseLog.Printf("[dir=%s] | [ignores=%v]", dir, outToIgnore)
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
