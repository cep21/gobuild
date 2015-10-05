package main

import (
	"github.com/BurntSushi/toml"
	"fmt"
	"os"
	"os/exec"
	"bufio"
	"golang.org/x/net/context"
	"io"
	"sync"
	"sync/atomic"
	"strings"
	"path/filepath"
	"sort"
	"flag"
)

type macro struct {
	Cmd *string `toml:"cmd"`
	Args []string `toml:"args"`
	Goget *string `toml:"goget"`
	OnlyAtRoot *bool `toml:"only-at-root"`
	IfFiles []string `toml:"if-files"`
}

func (m *macro) mergeFrom(from macro) {
	if from.Cmd != nil {
		m.Cmd = from.Cmd
	}
	if from.Args != nil {
		m.Args = from.Args
	}
	if from.Goget != nil{
		m.Goget = from.Goget
	}
	if from.OnlyAtRoot != nil {
		m.OnlyAtRoot = from.OnlyAtRoot
	}
	if from.IfFiles != nil {
		m.IfFiles = from.IfFiles
	}
}

type command struct {
	Macros []string `toml:"macros"`
	RunNext []string `toml:"run-next"`
}

func (g *gobuildInfo) StopCheck() (filenameMatcher, error) {
	return &directoryContainsMatcher{g.Vars["stop_loading_parent"].(string)}, nil
}

type directoryContainsMatcher struct {
	containsCheck []string
}

func (d *directoryContainsMatcher) Matches(filename string) bool {
	for _, c := range d.containsCheck {
		p := filepath.Join(filename, c)
		if _, err := os.Stat(p); err != nil {
			return true
		}
	}
	return false
}

type gobuildInfo struct {
	Macros map[string]macro `toml:"macro"`
	Vars map[string]interface{} `toml:"vars"`
	Commands map[string]command `toml:"cmd"`
}

func (g *gobuildInfo) buildfileName() string {
	return g.Vars["buildfileName"].(string)
}

// mergeFrom will merge into this build info data from another build info.  from will overwrite any
// information already in g, so it is the more important version
func (g *gobuildInfo) overrideFrom(from gobuildInfo) *gobuildInfo{
	// merge vars
	for k, v := range from.Vars {
		g.Vars[k] = v
	}
	// merge macros
	for macroName, macro := range from.Macros {
		oldMacro, exists := g.Macros[macroName]
		if !exists {
			g.Macros[macroName] = macro
			continue
		}
		oldMacro.mergeFrom(macro)
	}
	return g
}

func main() {
}

type Gobuild struct{}

func (g *Gobuild) main() error {
	g := gobuildInfo{}
	f := flagParser{
		flags: flag.NewFlagSet("flag_parser", flag.ErrHelp),
	}
	cmdToRun, paths, err := f.Parse(os.Args)
	if err != nil {
		return nil
	}
	t := TomlLoader{}
	primaryTemplate, err := t.fullyLoad(defaultDecodedTemplate)
	if err != nil {
		return err
	}

	loadedCommand, ok := primaryTemplate.Commands[cmdToRun]
	if !ok {
		errUnknownCommand(cmdToRun)
	}

	expandPaths(primaryTemplate, paths, ignorePaths, matchesPaths)
}

type errUnknownCommand string

func (e errUnknownCommand) Error() string {
	return fmt.Sprintf("unknown command %s", string(e))
}

type flagParser struct {
	flags flag.FlagSet
}

var defaultPaths = []string{"./..."}

func (f *flagParser) Parse(args []string) (string, []string, error){
	if err := f.flags.Parse(args); err != nil {
		return nil, nil, err
	}
	if f.flags.NArg() == 0 {
		return nil, defaultPaths, nil
	}
	return f.flags.Args()[0], f.flags.Args()[1:], nil
}

type TomlLoader struct {}

func (t *TomlLoader) fullyLoad(defaultTemplate *gobuildInfo) (*gobuildInfo, error) {
	// Merge toml files recursively in parent directories, ending with the defaulte template
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return t.loadInDir(defaultTemplate, filepath.Clean(dir))
}

type templateFinder struct {
	templatesForDirectories map[string]*gobuildInfo
	defaultTemplate *gobuildInfo
}

func (t *templateFinder) getTemplate(dirname string) (*gobuildInfo, error) {
	template, err := t.loadInDir(dirname)
	if err != nil {
		return nil, err
	}
	template, exists := t.templatesForDirectories[dirname]
	if exists {
		return template, nil
	}

}

func (t *templateFinder) loadInDir(dirname string) (*gobuildInfo, error) {
	template, exists := t.templatesForDirectories[dirname]
	if exists {
		return template, nil
	}
	if terminatingDirectoryName(dirname) {
		t.templatesForDirectories[dirname] = t.defaultTemplate
		return t.templatesForDirectories[dirname], nil
	}

	l, err := os.Stat(dirname)
	if err != nil {
		return t.defaultTemplate, nil
	}
	parent := filepath.Dir(dirname)
	if !l.IsDir() {
		return t.loadInDir(parent)
	}

	buildFileName := filepath.Join(dirname, t.defaultTemplate.buildfileName())
	l, err = os.Stat(buildFileName)
	if err == nil && !l.IsDir() {
		g := gobuildInfo{}
		toml.DecodeFile()
		t.templatesForDirectories[dirname] = parentInfo
		return parentInfo, nil
	}


	stopCheck, err := parentInfo.StopCheck()
	if err != nil {
		return nil, err
	}

	if stopCheck.

	parentInfo, err := t.loadInDir(parent)
	if err != nil {
		return nil, err
	}

}


func terminatingDirectoryName(dirname string) bool {
	return dirname == "" || dirname == "." || dirname == filepath.Separator
}

// The next directory with a build toml file or empty string
func (t *TomlLoader) nextBuildFile(dirname string, stopDir filenameMatcher, buildfileName string) string {
	for !terminatingDirectoryName(dirname) {
		buildFileName := filepath.Join(dirname, buildfileName)
		l, err := os.Stat(buildFileName)
		if err == nil && !l.IsDir() {
			return buildFileName
		}
		if stopDir(dirname) {
			return ""
		}
		dirname = filepath.Dir(dirname)
	}
	return ""
}

func mustTomlDecode(s string, into interface{}) toml.MetaData {
	m, err := toml.Decode(s, into)
	if err != nil {
		panic(err)
	}
	return m
}

var defaultDecodedTemplate gobuildInfo
var defaultDecodedTemplateMeta toml.MetaData

func init() {
	defaultDecodedTemplateMeta = mustTomlDecode(defaultTemplate, &defaultDecodedTemplate)
}

type filenameMatcher interface {
	Matches(filename string) bool
}

func expandPaths(rootTemplate *gobuildInfo, paths []string, ignorePaths filenameMatcher, matchesPaths filenameMatcher) ([]string, error) {
	files := make(map[string]struct{}, len(paths))
	templatesForDirectories := map[string]*gobuildInfo(len(paths) + 2)
	templatesForDirectories[""] = rootTemplate
	templatesForDirectories["."] = rootTemplate
	for _, path := range paths {
		if strings.HasSuffix(path, "/...") {
			root := filepath.Dir(path)
			if err := filepath.Walk(root, func(p string, i os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				finalPath := filepath.Clean(p)
				if ignorePaths.Matches(finalPath) {
					if i.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}

				if !i.IsDir() && matchesPaths.Matches(finalPath) {
					files[finalPath] = struct{}
				}
				return nil
			}); err != nil {
				return nil, err
			}
		} else {
			files[filepath.Clean(path)] = struct{}
		}
	}
	out := make([]string, 0, len(files))
	for d := range files {
		out = append(out, d)
	}
	sort.Strings(out)
	return out, nil
}


// cmdInDir represents a command to run inside a directory
type cmdInDir struct {
	cmd string
	args []string
	cwd string
	ifFiles []string
}

func streamLines(input io.Reader, into chan <- string, wg *sync.WaitGroup) {
	defer wg.Done()
	r := bufio.NewScanner(input)
	for r.Scan() {
		into <- r.Text()
	}
}

// Execute the command streaming lines of stdin and stdout.  Blocks until exec() is finished or the
// given context closes.  If the context closes early, it will try to kill the spawned connection.
func (c *cmdInDir) exec(ctx context.Context, stdoutStream chan <- string, stderrStream chan <- string) error {
	r := exec.Command(c.cmd, c.args...)
	r.Dir = c.cwd
	stdout, err := r.StdoutPipe()

	if err != nil {
		return err
	}
	stderr, err := r.StderrPipe()
	if err != nil {
		return err
	}
	wg := sync.WaitGroup{}
	wg.Add(2)
	go streamLines(stdout, stdoutStream, &wg)
	go streamLines(stderr, stderrStream, &wg)

	if err := r.Start(); err != nil {
		return err
	}

	doneWaiting := make(chan struct{})
	av := atomic.Value{}
	go func() {
		defer close(doneWaiting)
		wg.Wait()
		if err := r.Wait(); err != nil {
			fmt.Printf("Got error: %s\n", err.Error())
			av.Store(err)
			return
		}
	}()
	select {
	case <- ctx.Done():
		r.Process.Kill()
		<- doneWaiting
		return ctx.Err()
	case <- doneWaiting:
		if err := av.Load(); err != nil {
			return err.(error)
		}
		return nil
	}
}
