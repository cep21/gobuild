package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"io"

	"path/filepath"

	"github.com/cep21/gobuild/internal/golang.org/x/net/context"
)

type gobuildMain struct {
	args []string

	flags struct {
		verbose     bool
		verboseFile string
		chunkSize   int
		forceAbs    bool
	}

	tc         templateCache
	storageDir string

	verboseLog logger
	errLog     logger

	stderr io.Writer

	onClose []func() error
}

var mainInstance = gobuildMain{
	tc: templateCache{
		cache: make(map[string]*buildTemplate),
	},
	stderr: os.Stdout,
}

func init() {
	flag.BoolVar(&mainInstance.flags.verbose, "verbose", false, "Add verbose log to stderr")
	flag.StringVar(&mainInstance.flags.verboseFile, "verbosefile", "", "Will verbose log to a filename rather than stderr")
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
		if g.flags.verboseFile != "" {
			verboseFile, err := os.Create(g.flags.verboseFile)
			if err != nil {
				return wraperr(err, "cannot create verbose file %s", g.flags.verboseFile)
			}
			vlog = verboseFile
			g.onClose = append(g.onClose, verboseFile.Close)
		}
	}
	g.verboseLog = log.New(vlog, "[gobuild-verbose]", log.LstdFlags|log.Lshortfile)
	g.errLog = log.New(os.Stderr, "[gobuild-err]", log.LstdFlags|log.Lshortfile)
	g.tc.verboseLog = g.verboseLog

	var err error
	g.storageDir, err = g.storageDirectory()
	if err != nil {
		return wraperr(err, "cannot create test storage directory")
	}
	g.verboseLog.Printf("Storing results to %s", g.storageDir)

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
	testDirs, err := dirsWithFileGob(dirs, "*.go")
	if err != nil {
		return wraperr(err, "cannot find *.go files in dirs")
	}
	c := gometalinterCmd{
		verboseLog: g.verboseLog,
		errLog:     g.errLog,
		metaOutput: &myselfOutput{&nopCloseWriter{os.Stderr}},
		dirsToLint: testDirs,
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
	htmlOut, err := os.Create(filepath.Join(g.storageDir, "coverage.html"))
	if err != nil {
		return wraperr(err, "cannot create coverage html file")
	}
	c := duplCmd{
		verboseLog: g.verboseLog,
		dirs:       dirs,
		consoleOut: os.Stdout,
		htmlOut:    htmlOut,
		tmpl:       tmpl,
	}
	return multiErr([]error{c.Run(ctx), htmlOut.Close()})
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

func (g *gobuildMain) storageDirectory() (string, error) {
	tmpl, err := g.tc.loadInDir(".")
	if err != nil {
		return "", wraperr(err, "cannot load root template directory")
	}
	fromEnv := os.Getenv(tmpl.varStr("artifactsEnv"))
	if fromEnv != "" {
		return fromEnv, nil
	}
	artifactDir := filepath.Join(os.TempDir(), "gobuild")
	if err := os.RemoveAll(artifactDir); err != nil {
		return "", wraperr(err, "Cannot clean directory %s", artifactDir)
	}
	if err := os.MkdirAll(artifactDir, 0777); err != nil {
		return "", wraperr(err, "Cannot create directory %s", artifactDir)
	}

	return artifactDir, nil
}

func (g *gobuildMain) test(ctx context.Context, dirs []string) error {
	testDirs, err := dirsWithFileGob(dirs, "*.go")
	if err != nil {
		return wraperr(err, "cannot find *.go files in dirs")
	}

	fullOut, err := os.Create(filepath.Join(g.storageDir, "full_coverage_output.cover.txt"))
	if err != nil {
		return wraperr(err, "cannot create full coverage profile file")
	}
	c := goCoverageCheck{
		dirs:               testDirs,
		cache:              &g.tc,
		coverProfileOutTo:  inDirStreamer(g.storageDir, ".cover.txt"),
		testStdoutOutputTo: &myselfOutput{&nopCloseWriter{os.Stdout}},
		testStderrOutputTo: &myselfOutput{&nopCloseWriter{os.Stderr}},
		requiredCoverage:   0,
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

func (g *gobuildMain) Close() error {
	errs := make([]error, 0, len(g.onClose))
	for _, f := range g.onClose {
		if err := f(); err != nil {
			errs = append(errs, err)
		}
	}
	return multiErr(errs)
}

func (g *gobuildMain) main() error {
	defer func() {
		if err := g.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Cannot close mainInstance: %s\n", err.Error())
		}
	}()

	if err := g.parseFlags(); err != nil {
		return wraperr(err, "cannot parse flags")
	}
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
