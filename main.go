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
	"regexp"
)

type macro struct {
	Cmd *string `toml:"cmd"`
	Args []string `toml:"args"`
	Goget *string `toml:"goget"`
	CrossDirectory *bool `toml:"cross-directory"`
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
	if from.CrossDirectory != nil {
		m.CrossDirectory = from.CrossDirectory
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

func (g *gobuildInfo) command(name string) (command, bool) {
	c, exists := g.Commands[name]
	return c, exists
}

func (g *gobuildInfo) ignoredPaths() (filenameMatcher, error) {
	ignoreVars := g.Vars["ignoreDirs"].([]string)
	ret := make([]filenameMatcher, 0, len(ignoreVars))
	for _, dir := range ignoreVars {
		reg, err := regexp.Compile(dir)
		if err != nil {
			return nil, err
		}
		ret = append(ret, endingMatchesRegexMatcher{
			reg:reg,
		})
	}
	return anyMatches(ret), nil
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

type Gobuild struct{
	templateMap templateFinder
}

type GroupToRun struct {
	cwd string
	files []string
	tmpl *gobuildInfo
}

func (g *Gobuild) main() error {
	f := flagParser{
		flags: flag.NewFlagSet("flag_parser", flag.ErrHelp),
	}
	cmdToRun, paths, err := f.Parse(os.Args)
	if err != nil {
		return nil
	}

	filesToCheck, err := expandPaths(g.templateMap, paths)
	if err != nil {
		return err
	}

	// Group every file by directory
	groupsToRun, err := groupFiles(filesToCheck, g.templateMap)

	// Make sure the command to run is defined for every file you want to check.
	if err := commandExistsForPaths(cmdToRun, groupsToRun, g.templateMap); err != nil {
		return err
	}

	installs, err := getInstallCommands(groupsToRun, cmdToRun)
	if err != nil {
		return err
	}

	installs = condenseInstallCommands(installs)

	ctx := context.Background()

	// First step, setup installs if needed
	for _, i := range installs {
		if err := i.install(ctx); err != nil {
			return err
		}
	}


	return nil
}

func condenseInstallCommands(installs []*installCommand) []*installCommand {
	ret := make([]*installCommand, 0, len(installs))
	allGoGetPaths := make([]string, 0, len(installs))

	for _, i := range installs {
		if i.shouldInstall() && i.goGetPath != "" {
			allGoGetPaths = append(allGoGetPaths, i.goGetPath)
		} else if i.shouldInstall() {
			ret = append(ret, i)
		}
	}
	if len(allGoGetPaths) != 0 {
		ret = append(ret, &installCommand{
			installArgs: append([]string{"go", "get"}, allGoGetPaths...),
		})
	}
	return ret
}

type installCommand struct {
	checkExists string
	installArgs []string
	goGetPath string
}

func (i *installCommand) shouldInstall() bool {
	path, err := exec.LookPath(i.checkExists)
	return path == "" || err != nil
}

func (i *installCommand) install(ctx context.Context) error {
	cmd := cmdInDir{
		cmd: i.installArgs[0],
		args: i.installArgs[1:],
		cwd: "",
	}
	stderr := make(chan string)
	stdout := make(chan string)
	wg := sync.WaitGroup{}
	wg.Add(2)
	defer wg.Wait()
	defer close(stderr)
	defer close(stdout)
	go streamInto(stderr, os.Stderr, &wg)
	go streamInto(stdout, os.Stderr, &wg)

	return cmd.exec(ctx, stdout, stderr)
}

func installsForTemplate(arg string, t *gobuildInfo) (map[string]*installCommand, error) {
	installMap := make(map[string]*installCommand)
	cmd, err := t.command(arg)
	if err != nil {
		return nil, err
	}
	for _, macroName := range cmd.Macros {
		m := t.Macros[macroName]
		installMap[*m.Cmd] = &installCommand{
			checkExists: *m.Cmd,
			installArgs: []string{"go", "get", "-u", *m.Goget},
			goGetPath: *m.Goget,
		}
	}
	for _, n := range cmd.RunNext {
		m, err := installsForTemplate(n, t)
		if err != nil {
			return nil, err
		}
		for k, v := range m {
			installMap[k] = v
		}
	}
	return installMap
}

func getExecCommands(cmd string, groupsToRun []*GroupToRun) ([][]*cmdInDir, error) {
	phases := [][]*cmdInDir{}
	for _, g := range groupsToRun {
		cmdToRun, exists := g.tmpl.command(cmd)
		if !exists {
			return nil, errUnknownCommand(cmd)
		}
		for _, m := range cmdToRun.Macros {
		}
	}
	return phases
}

func getInstallCommands(groupsToRun []*GroupToRun, arg string) ([]*installCommand, error) {
	installMap := make(map[string]*installCommand)
	for _, g := range groupsToRun {
		m, err := installsForTemplate(arg, g.tmpl)
		if err != nil {
			return nil, err
		}
		for k, v := range m {
			installMap[k] = v
		}
	}
	ret := make([]*installCommand, 0, len(installMap))
	for _, m := range installMap {
		ret = append(ret, m)
	}
	return ret, nil
}

func groupFiles(paths []string, templateMap templateFinder) ([]*GroupToRun, error) {
	ret := make(map[string]*GroupToRun)
	for _, p := range paths {
		dir := filepath.Dir(p)
		if g, exists := ret[dir]; exists {
			g.files = append(g.files, p)
			continue
		}

		t, err := templateMap.loadInDir(p)
		if err != nil {
			return nil, err
		}
		ret[dir] = &GroupToRun{
			cwd: dir,
			files: []string{p},
			tmpl: t,
		}
	}
	r := make([]*GroupToRun, 0, len(ret))
	for _, v := range ret {
		r = append(r, v)
	}
	return r
}

func commandExistsForPaths(cmd string, paths[]*GroupToRun, templateMap templateFinder) error {
	for _, p := range paths {
		t, err := templateMap.loadInDir(p.cwd)
		if err != nil {
			return err
		}
		macro, exists := t.command(cmd)
		if !exists {
			return errUnknownCommand(cmd)
		}
		for _, m := range macro.Macros {
			if _, exists := t.Macros[m]; !exists {
				return errUnknownCommand(m)
			}
		}
	}
	return nil
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

type templateFinder struct {
	templatesForDirectories map[string]*gobuildInfo
	defaultTemplate *gobuildInfo
}

func (t *templateFinder) loadInDir(dirname string) (*gobuildInfo, error) {
	if template, exists := t.templatesForDirectories[dirname]; exists {
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

	// At this point, we know dirname is a directory

	buildFileName := filepath.Join(dirname, t.defaultTemplate.buildfileName())
	l, err = os.Stat(buildFileName)
	thisDirectoryBuildInfo, stopCheck, err := func() (*gobuildInfo, filenameMatcher, error) {
		if err == nil && !l.IsDir() {
			retInfo := &gobuildInfo{}
			if _, err := toml.DecodeFile(buildFileName, retInfo); err != nil {
				return nil, nil, err
			}
			stopCheck, err := retInfo.StopCheck()
			return retInfo, stopCheck, err
		} else {
			sc, err := t.defaultTemplate.StopCheck()
			return nil, sc, err
		}
	}()
	if err != nil {
		return nil, err
	}

	parentInfo, err := func() (*gobuildInfo, error) {
		if stopCheck.Matches(dirname) {
			return t.defaultTemplate, nil
		} else {
			return t.loadInDir(parent)
		}
	}()
	if err != nil {
		return nil, err
	}
	if thisDirectoryBuildInfo == nil {
		t.templatesForDirectories[dirname] = parentInfo
	} else {
		t.templatesForDirectories[dirname] = (&gobuildInfo{}).overrideFrom(parentInfo).overrideFrom(thisDirectoryBuildInfo)
	}
	return t.templatesForDirectories[dirname], nil
}


func terminatingDirectoryName(dirname string) bool {
	return dirname == "" || dirname == "." || dirname == filepath.Separator
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

type endingMatchesRegexMatcher struct {
	reg *regexp.Regexp
}

func (e *endingMatchesRegexMatcher) Matches(filename string) bool {
	return e.reg.MatchString(filepath.Base(filename))
}

type anyMatches []filenameMatcher

func (a anyMatches) Matches(filename string) bool {
	for _, m := range a {
		if m.Matches(filename) {
			return true
		}
	}
	return false
}

func expandPaths(templateMap templateFinder, paths []string) ([]string, error) {
	// ignorePaths filenameMatcher
	files := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		if strings.HasSuffix(path, "/...") {
			if err := filepath.Walk(filepath.Dir(path), func(p string, i os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				finalPath := filepath.Clean(p)
				template, err := templateMap.loadInDir(p)
				if err != nil {
					return err
				}
				ignorePaths, err := template.ignoredPaths()
				if err != nil {
					return err
				}
				if ignorePaths.Matches(finalPath) {
					if i.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}

				if !i.IsDir() {
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
}

func streamLines(input io.Reader, into chan <- string, wg *sync.WaitGroup) {
	defer wg.Done()
	r := bufio.NewScanner(input)
	for r.Scan() {
		into <- r.Text()
	}
}

func streamInto(from <- chan string, into io.Writer, wg *sync.WaitGroup) {
	defer wg.Done()
	for l := range from {
		if l != "" {
			io.WriteString(into, l)
			io.WriteString(into, "\n")
		}
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
