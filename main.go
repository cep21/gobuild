package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"golang.org/x/net/context"
)

type gobuildMain struct {
	args []string

	flags struct {
		verbose   bool
		chunkSize int
		forceAbs  bool
	}

	verboseLog logger
	errLog     logger
}

var mainInstance = gobuildMain{}

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
	c := fixCmd{
		dirs:       dirs,
		chunkSize:  g.flags.chunkSize,
		verboseOut: g.verboseLog,
		errOut:     g.errLog,
	}
	return c.Run(ctx)
}

func (g *gobuildMain) main() error {
	if err := g.parseFlags(); err != nil {
		return wraperr(err, "cannot parse flags")
	}
	ctx := context.Background()

	tc := templateCache{
		cache: make(map[string]*buildTemplate),
	}
	pe := pathExpansion{
		forceAbs: g.flags.forceAbs,
		log:      g.verboseLog,
		template: &tc,
	}

	cmdMap := map[string]func(context.Context, []string) error{
		"fix": g.fix,
	}

	cmd, args := g.getArgs()
	f, exists := cmdMap[cmd]
	if !exists {
		return fmt.Errorf("unknown command %s", cmd)
	}
	dirs, err := pe.expandPaths(args)
	if err != nil {
		return wraperr(err, "cannot expand paths %s", strings.Join(args, ","))
	}
	if err := f(ctx, dirs); err != nil {
		return wraperr(err, "unable to execute command %s", cmd)
	}
	return nil
}
