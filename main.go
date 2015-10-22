package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"io"

	"github.com/cep21/gobuild/internal/golang.org/x/net/context"
)

type gobuildMain struct {
	args []string

	flags struct {
		verbose   bool
		chunkSize int
		forceAbs  bool
	}

	tc templateCache

	verboseLog logger
	errLog     logger

	stderr io.Writer
}

var mainInstance = gobuildMain{
	tc: templateCache{
		cache: make(map[string]*buildTemplate),
	},
	stderr: os.Stdout,
}

func init() {
	flag.BoolVar(&mainInstance.flags.verbose, "verbose", false, "Add verbose log to stderr")
	flag.IntVar(&mainInstance.flags.chunkSize, "chunksize", 250, "size to chunk xargs into")
	flag.BoolVar(&mainInstance.flags.forceAbs, "abs", false, "will force abs paths for ... dirs")
}

func main() {
	flag.Parse()
	mainInstance.args = flag.Args()
	if err := mainInstance.main(); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}
}

func (g *gobuildMain) parseFlags() error {
	vlog := ioutil.Discard
	if g.flags.verbose {
		vlog = os.Stderr
	}
	g.verboseLog = log.New(vlog, "[gobuild-verbose]", 0)
	g.errLog = log.New(os.Stderr, "[gobuild-err]", 0)
	return nil
}

func (g *gobuildMain) getArgs() (string, []string) {
	if len(g.args) == 0 {
		return "check", []string{"./..."}
	}
	if len(g.args) == 1 {
		return g.args[0], []string{"./..."}
	}
	return g.args[0], g.args[1:]
}

func (g *gobuildMain) fix(ctx context.Context, dirs []string) error {
	if err := g.install(ctx, dirs); err != nil {
		return wraperr(err, "cannot install subcommands")
	}
	c := fixCmd{
		dirs:       dirs,
		chunkSize:  g.flags.chunkSize,
		verboseOut: g.verboseLog,
		errOut:     g.errLog,
	}
	return c.Run(ctx)
}

func (g *gobuildMain) lint(ctx context.Context, dirs []string) error {
	if err := g.install(ctx, dirs); err != nil {
		return wraperr(err, "cannot install subcommands")
	}
	c := gometalinterCmd{
		verboseLog: g.verboseLog,
		errLog:     g.errLog,
		metaOutput: &myselfOutput{&nopCloseWriter{os.Stderr}},
		dirsToLint: dirs,
		cache:      &g.tc,
	}
	return c.Run(ctx)
}

func (g *gobuildMain) build(ctx context.Context, dirs []string) error {
	buildableDirs, err := dirsWithFileGob(dirs, "*.go")
	if err != nil {
		return wraperr(err, "cannot find *.go files in dirs")
	}
	c := cmdBuild{
		verboseLog: g.verboseLog,
		errorLog:   g.errLog,
		cmdStdout:  &myselfOutput{&nopCloseWriter{os.Stdout}},
		cmdStderr:  &myselfOutput{&nopCloseWriter{os.Stderr}},
		dirs:       buildableDirs,
		cache:      &g.tc,
	}
	return c.Run(ctx)
}

func (g *gobuildMain) dupl(ctx context.Context, dirs []string) error {
	if err := g.install(ctx, dirs); err != nil {
		return wraperr(err, "cannot install subcommands")
	}
	tmpl, err := g.tc.loadInDir(".")
	if err != nil {
		return wraperr(err, "cannot load root dir template")
	}
	c := duplCmd{
		verboseLog: g.verboseLog,
		dirs:       dirs,
		consoleOut: os.Stdout,
		htmlOut:    ioutil.Discard,
		tmpl:       tmpl,
	}
	return c.Run(ctx)
}

func (g *gobuildMain) install(ctx context.Context, dirs []string) error {
	tmpl, err := g.tc.loadInDir(".")
	if err != nil {
		return wraperr(err, "cannot load root dir template")
	}
	c := installCmd{
		forceReinstall: false,
		verboseLog:     g.verboseLog,
		errLog:         g.errLog,
		stdoutOutput:   os.Stdout,
		stderrOutput:   os.Stderr,
		tmpl:           tmpl,
	}
	return c.Run(ctx)
}

func (g *gobuildMain) test(ctx context.Context, dirs []string) error {
	testDirs, err := dirsWithFileGob(dirs, "*.go")
	if err != nil {
		return wraperr(err, "cannot find *.go files in dirs")
	}
	fullOut, err := os.Create("/tmp/a/full_coverage_output.cover")
	if err != nil {
		return wraperr(err, "cannot create full coverage profile file")
	}
	c := goCoverageCheck{
		dirs:               testDirs,
		cache:              &g.tc,
		coverProfileOutTo:  inDirStreamer("/tmp/a", ".cover"),
		testStdoutOutputTo: &myselfOutput{&nopCloseWriter{os.Stdout}},
		testStderrOutputTo: &myselfOutput{&nopCloseWriter{os.Stderr}},
		requiredCoverage:   1,
		verboseLog:         g.verboseLog,
		errLog:             g.errLog,
		fullCoverageOutput: fullOut,
	}
	e1 := c.Run(ctx)
	e2 := fullOut.Close()
	return multiErr([]error{e1, e2})
}

func (g *gobuildMain) check(ctx context.Context, dirs []string) error {
	buildErr := g.build(ctx, dirs)
	lintErr := g.lint(ctx, dirs)
	duplErr := g.dupl(ctx, dirs)
	testErr := g.test(ctx, dirs)
	return multiErr([]error{buildErr, lintErr, duplErr, testErr})
}

func (g *gobuildMain) list(ctx context.Context, dirs []string) error {
	g.verboseLog.Printf("len(dirs) = %d", len(dirs))
	fmt.Printf("%s\n", strings.Join(dirs, "\n"))
	return nil
}

func (g *gobuildMain) main() error {
	if err := g.parseFlags(); err != nil {
		return wraperr(err, "cannot parse flags")
	}
	g.tc.verboseLog = g.verboseLog
	ctx := context.Background()

	pe := pathExpansion{
		forceAbs: g.flags.forceAbs,
		log:      g.verboseLog,
		template: &g.tc,
	}

	cmdMap := map[string]func(context.Context, []string) error{
		"fix":     g.fix,
		"lint":    g.lint,
		"list":    g.list,
		"build":   g.build,
		"test":    g.test,
		"dupl":    g.dupl,
		"install": g.install,
		"check":   g.check,
	}

	cmd, args := g.getArgs()
	f, exists := cmdMap[cmd]
	if !exists {
		fmt.Fprintf(g.stderr, "Unknown command %s\nValid commands:\n", cmd)
		for k := range cmdMap {
			fmt.Fprintf(g.stderr, "  %s\n", k)
		}
		return fmt.Errorf("unknown command %s", cmd)
	}
	dirs, err := pe.expandPaths(args)
	if err != nil {
		return wraperr(err, "cannot expand paths %s", strings.Join(args, ","))
	}
	if err := f(ctx, dirs); err != nil {
		return wraperr(err, "Failure in command %s", cmd)
	}
	return nil
}
