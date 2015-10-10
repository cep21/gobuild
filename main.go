package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/BurntSushi/toml"
	"golang.org/x/net/context"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

type macro struct {
	Cmd            *string  `toml:"cmd"`
	Args           []string `toml:"args"`
	Goget          *string  `toml:"goget"`
	CrossDirectory *bool    `toml:"cross-directory"`
	IfFiles        []string `toml:"if-files"`
	AppendFiles    bool     `toml:"append-files"`
}

func (m *macro) ifFilesMatcher() (filenameMatcher, error) {
	if m.IfFiles == nil || len(m.IfFiles) == 0 {
		return trueMatcher{}, nil
	}
	r := []filenameMatcher{}
	for _, f := range m.IfFiles {
		reg, err := regexp.Compile(f)
		if err != nil {
			return nil, err
		}
		r = append(r, &endingMatchesRegexMatcher{reg: reg})
	}
	return anyMatches(r), nil
}

func (m *macro) mergeFrom(from macro) {
	if from.Cmd != nil {
		m.Cmd = from.Cmd
	}
	if from.Args != nil {
		m.Args = from.Args
	}
	if from.Goget != nil {
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
	Macros  []string `toml:"macros"`
	RunNext []string `toml:"run-next"`
}

func (g *gobuildInfo) StopCheck() (filenameMatcher, error) {
	return &directoryContainsMatcher{arrIntToarrStr(g.Vars["stop_loading_parent"].([]interface{}))}, nil
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
	Macros   map[string]macro       `toml:"macro"`
	Vars     map[string]interface{} `toml:"vars"`
	Commands map[string]command     `toml:"cmd"`
}

func (g *gobuildInfo) buildfileName() string {
	return g.Vars["buildfileName"].(string)
}

func (g *gobuildInfo) parrallelBuildCount() int64 {
	return g.Vars["parallelBuildCount"].(int64)
}

func (g *gobuildInfo) varAsString(name string) (string, bool) {
	v, exists := g.Vars[name]
	if !exists {
		return "", false
	}
	switch v.(type) {
	case int:
	case int64:
		return fmt.Sprintf("%d", v), true
	case float64:
		return fmt.Sprintf("%f", v), true
	}
	return fmt.Sprintf("%s", v), true
}

func (g *gobuildInfo) command(name string) (command, bool) {
	c, exists := g.Commands[name]
	return c, exists
}

func arrIntToarrStr(ints []interface{}) []string {
	r := make([]string, 0, len(ints))
	for _, i := range ints {
		r = append(r, i.(string))
	}
	return r
}

func (g *gobuildInfo) ignoredPaths() (filenameMatcher, error) {
	ignoreVars := arrIntToarrStr(g.Vars["ignoreDirs"].([]interface{}))
	ret := make([]filenameMatcher, 0, len(ignoreVars))
	for _, dir := range ignoreVars {
		reg, err := regexp.Compile(dir)
		if err != nil {
			return nil, err
		}
		ret = append(ret, &endingMatchesRegexMatcher{
			reg: reg,
		})
	}
	return anyMatches(ret), nil
}

// mergeFrom will merge into this build info data from another build info.  from will overwrite any
// information already in g, so it is the more important version
func (g *gobuildInfo) overrideFrom(from gobuildInfo) *gobuildInfo {
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
	m := gobuild{
		templateMap: templateFinder{
			templatesForDirectories: make(map[string]*gobuildInfo),
			defaultTemplate:         &defaultDecodedTemplate,
		},
		log: log.New(ioutil.Discard, "", 0),
	}
	if err := m.main(); err != nil {
		panic(err)
	}
}

type gobuild struct {
	templateMap templateFinder
	log         *log.Logger
}

type groupToRun struct {
	cwd   string
	files []string
	tmpl  *gobuildInfo
}

func (g *gobuild) main() error {
	f := flagParser{
		flags: flag.NewFlagSet("flag_parser", flag.ExitOnError),
	}
	cmdToRun, paths, err := f.Parse(g.log, os.Args)
	if err != nil {
		return nil
	}
	g.log.Printf("Command:-%s- Paths: -%s-\n", cmdToRun, paths)

	filesToCheck, err := expandPaths(g.log, g.templateMap, paths)
	if err != nil {
		return err
	}

	g.log.Printf("Checking files %s\n", strings.Join(filesToCheck, ","))

	// Group every file by directory
	groupsToRun, err := groupFiles(filesToCheck, g.templateMap)
	if err != nil {
		return err
	}

	g.log.Printf("Groups to run %#v\n", groupsToRun)

	// Make sure the command to run is defined for every file you want to check.
	if err := commandExistsForPaths(cmdToRun, groupsToRun, g.templateMap); err != nil {
		log.Printf("Command %s not defined\n", cmdToRun)
		return err
	}

	installs, err := getInstallCommands(groupsToRun, cmdToRun)
	if err != nil {
		return err
	}

	g.log.Printf("installs %+v\n", installs)

	installs = condenseInstallCommands(installs)

	g.log.Printf("installs %+v\n", installs)

	ctx := context.Background()

	// First step, setup installs if needed
	for _, i := range installs {
		if err := i.install(g.log, ctx); err != nil {
			return err
		}
	}

	execCmd, err := getExecCommands(g.log, cmdToRun, groupsToRun)
	if err != nil {
		return err
	}

	g.log.Printf("execCmd %+v\n", execCmd)

	primaryTemplate, err := g.templateMap.loadInDir(".")
	if err != nil {
		return err
	}

	runPhases(ctx, g.log, primaryTemplate, execCmd)
	return nil
}

func runPhases(ctx context.Context, log *log.Logger, primaryTemplate *gobuildInfo, execCmd [][]*cmdToProcess) {
	output := make(chan *errorResult)
	outputWaiting := sync.WaitGroup{}
	outputWaiting.Add(1)
	go drainOutputPipeline(output, &outputWaiting)
	for pi, phase := range execCmd {
		log.Printf("Phase %d\n", pi)
		executionPipeline := make(chan *cmdToProcess, 1024)
		wg := sync.WaitGroup{}
		numberOfBuilds := primaryTemplate.parrallelBuildCount()
		wg.Add(int(numberOfBuilds))
		log.Printf("Running %d builds\n", numberOfBuilds)
		for i := int64(0); i < numberOfBuilds; i++ {
			go drainExecutionPipeline(ctx, log, executionPipeline, output, &wg)
		}
		for _, cmd := range phase {
			executionPipeline <- cmd
		}
		close(executionPipeline)
		wg.Wait()
	}

	close(output)
	outputWaiting.Wait()
}

func drainOutputPipeline(outputs <-chan *errorResult, wg *sync.WaitGroup) {
	defer wg.Done()
	for p := range outputs {
		fmt.Println(p.String())
	}
}

func drainExecutionPipeline(ctx context.Context, log *log.Logger, ch <-chan *cmdToProcess, out chan<- *errorResult, wg *sync.WaitGroup) {
	defer wg.Done()
	for p := range ch {
		log.Printf("Running %s", p.cmd)
		procRunning := sync.WaitGroup{}
		log.Printf("I would run %v %s\n", p, p.cmd)
		stdoutStream := make(chan string, 1024)
		stderrStream := make(chan string, 1024)
		procRunning.Add(2)
		go processInputStream(stdoutStream, out, p.stdoutProcessor, &procRunning)
		go processInputStream(stderrStream, out, p.stderrProcessor, &procRunning)
		if e := p.cmd.exec(ctx, log, stdoutStream, stderrStream); e != nil {
			if ep := p.execCodeProcessor.OnExit(e); ep != nil {
				out <- ep
			}
		}
		close(stdoutStream)
		close(stderrStream)
		procRunning.Wait()
		log.Printf("Done Running %s", p.cmd)
	}
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
	goGetPath   string
}

func (i *installCommand) shouldInstall() bool {
	path, err := exec.LookPath(i.checkExists)
	return path == "" || err != nil
}

func (i *installCommand) install(log *log.Logger, ctx context.Context) error {
	cmd := cmdInDir{
		cmd:  i.installArgs[0],
		args: i.installArgs[1:],
		cwd:  "",
	}
	stderr := make(chan string)
	stdout := make(chan string)
	wg := sync.WaitGroup{}
	wg.Add(2)
	defer wg.Wait()
	defer close(stderr)
	defer close(stdout)
	go streamInto(log, stderr, os.Stderr, &wg)
	go streamInto(log, stdout, os.Stderr, &wg)

	return cmd.exec(ctx, log, stdout, stderr)
}

func installsForTemplate(arg string, t *gobuildInfo) (map[string]*installCommand, error) {
	installMap := make(map[string]*installCommand)
	cmd, exists := t.command(arg)
	if !exists {
		return nil, errUnknownCommand(arg)
	}
	for _, macroName := range cmd.Macros {
		m := t.Macros[macroName]
		if m.Goget != nil {
			installMap[*m.Cmd] = &installCommand{
				checkExists: *m.Cmd,
				installArgs: []string{"go", "get", "-u", *m.Goget},
				goGetPath:   *m.Goget,
			}
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
	return installMap, nil
}

func replaceArgs(args []string, g *gobuildInfo) []string {
	ret := make([]string, 0, len(args))
	for _, r := range args {
		varName := varNameForString(r)
		if varName == "" {
			ret = append(ret, r)
		}
		if varVal, exists := g.varAsString(varName); exists {
			ret = append(ret, varVal)
		}
	}
	return ret
}

func varNameForString(s string) string {
	if len(s) <= 2 {
		return ""
	}
	if s[0] == '{' && s[len(s)-1] == '}' {
		return s[1 : len(s)-1]
	}
	return ""
}

type outputProcessor interface {
	ParseError(line string) *errorResult
}

type echoOutputProcessor struct {
	checkName string
}

func (e echoOutputProcessor) ParseError(line string) *errorResult {
	if line == "" {
		return nil
	}
	return &errorResult{
		path:    "hi",
		message: line + "(" + e.checkName + ")",
	}
}

type exitProcessor interface {
	OnExit(err error) *errorResult
}

type ignoreExitCode struct{}

func (e ignoreExitCode) OnExit(err error) *errorResult {
	return nil
}

func processInputStream(ch <-chan string, out chan<- *errorResult, p outputProcessor, wg *sync.WaitGroup) {
	defer wg.Done()
	for s := range ch {
		if e := p.ParseError(s); e != nil {
			out <- e
		}
	}
}

type errorResult struct {
	path     string
	line     int
	col      int
	severity severity
	message  string
}

type severity int

const (
//	warning severity = iota
)

func (s severity) String() string {
	return "warning"
}

func (e *errorResult) String() string {
	return fmt.Sprintf("%s:%d:%d:%s:%s", e.path, e.line, e.col, e.severity.String(), e.message)
}

type cmdToProcess struct {
	cmd               *cmdInDir
	stdoutProcessor   outputProcessor
	stderrProcessor   outputProcessor
	execCodeProcessor exitProcessor
}

func rootPhaseForMacro(log *log.Logger, g *groupToRun, cmdToRun command) ([]*cmdToProcess, error) {
	ret := make([]*cmdToProcess, 0, len(cmdToRun.Macros))
	for _, m := range cmdToRun.Macros {
		macro := g.tmpl.Macros[m]
		log.Printf("Looking at macro %+v", macro)
		ifFilesMatcher, err := macro.ifFilesMatcher()
		if err != nil {
			return nil, err
		}
		matchedFiles := make([]string, 0, len(g.files))
		for _, file := range g.files {
			if ifFilesMatcher.Matches(file) {
				matchedFiles = append(matchedFiles, file)
			}
		}
		if len(matchedFiles) == 0 {
			log.Printf("No matched files")
			continue
		}
		cmd := cmdInDir{
			cmd:  *macro.Cmd,
			args: replaceArgs(macro.Args, g.tmpl),
			cwd:  g.cwd,
		}
		if macro.AppendFiles {
			cmd.args = append(cmd.args, matchedFiles...)
		}
		ret = append(ret, &cmdToProcess{
			cmd: &cmd,
			stdoutProcessor: echoOutputProcessor{
				checkName: m,
			},
			stderrProcessor: echoOutputProcessor{
				checkName: m,
			},
			execCodeProcessor: ignoreExitCode{},
		})
	}
	return ret, nil
}

func getExecCommands(log *log.Logger, cmd string, groupsToRun []*groupToRun) ([][]*cmdToProcess, error) {
	phases := [][]*cmdToProcess{make([]*cmdToProcess, 0, len(groupsToRun))}
	for _, g := range groupsToRun {
		cmdToRun, exists := g.tmpl.command(cmd)
		if !exists {
			return nil, errUnknownCommand(cmd)
		}

		rootPhases, err := rootPhaseForMacro(log, g, cmdToRun)
		if err != nil {
			return nil, err
		}
		phases[0] = append(phases[0], rootPhases...)

		for _, runNext := range cmdToRun.RunNext {
			nextPhase, err := getExecCommands(log, runNext, []*groupToRun{g})
			if err != nil {
				return nil, err
			}
			for i, phase := range nextPhase {
				phaseIndex := i + 1
				if len(phases) == phaseIndex {
					phases = append(phases, []*cmdToProcess{})
				}
				phases[phaseIndex] = append(phases[phaseIndex], phase...)
			}
		}
	}
	return phases, nil
}

func getInstallCommands(groupsToRun []*groupToRun, arg string) ([]*installCommand, error) {
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

func groupFiles(paths []string, templateMap templateFinder) ([]*groupToRun, error) {
	ret := make(map[string]*groupToRun)
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
		ret[dir] = &groupToRun{
			cwd:   dir,
			files: []string{p},
			tmpl:  t,
		}
	}
	r := make([]*groupToRun, 0, len(ret))
	for _, v := range ret {
		r = append(r, v)
	}
	return r, nil
}

func commandExistsForPaths(cmd string, paths []*groupToRun, templateMap templateFinder) error {
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
	flags *flag.FlagSet
}

var defaultPaths = []string{"./..."}

func (f *flagParser) Parse(log *log.Logger, args []string) (string, []string, error) {
	log.Printf("Parsing %s\n", strings.Join(args, " "))
	if err := f.flags.Parse(args); err != nil {
		return "", nil, err
	}
	if f.flags.NArg() <= 1 {
		return "", defaultPaths, nil
	}
	if f.flags.NArg() == 2 {
		return f.flags.Args()[1], defaultPaths, nil
	}
	return f.flags.Args()[1], f.flags.Args()[2:], nil
}

type templateFinder struct {
	templatesForDirectories map[string]*gobuildInfo
	defaultTemplate         *gobuildInfo
}

func (t *templateFinder) getBuildInfo(buildFileName string) (*gobuildInfo, filenameMatcher, error) {
	l, err := os.Stat(buildFileName)
	if err == nil && !l.IsDir() {
		retInfo := &gobuildInfo{}
		if _, err = toml.DecodeFile(buildFileName, retInfo); err != nil {
			return nil, nil, err
		}
		stopCheck, stopError := retInfo.StopCheck()
		return retInfo, stopCheck, stopError
	}
	sc, err2 := t.defaultTemplate.StopCheck()
	return nil, sc, err2
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
	thisDirectoryBuildInfo, stopCheck, err := t.getBuildInfo(buildFileName)
	if err != nil {
		return nil, err
	}

	parentInfo, err := func() (*gobuildInfo, error) {
		if stopCheck.Matches(dirname) {
			return t.defaultTemplate, nil
		}
		return t.loadInDir(parent)
	}()
	if err != nil {
		return nil, err
	}
	if thisDirectoryBuildInfo == nil {
		t.templatesForDirectories[dirname] = parentInfo
	} else {
		t.templatesForDirectories[dirname] = (&gobuildInfo{}).overrideFrom(*parentInfo).overrideFrom(*thisDirectoryBuildInfo)
	}
	return t.templatesForDirectories[dirname], nil
}

func terminatingDirectoryName(dirname string) bool {
	return dirname == "" || dirname == "." || dirname == string(filepath.Separator)
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

type trueMatcher struct{}

func (t trueMatcher) Matches(filename string) bool {
	return true
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

func walkCallback(log *log.Logger, templateMap templateFinder, files map[string]struct{}) filepath.WalkFunc {
	return func(p string, i os.FileInfo, err error) error {
		log.Printf("At %s\n", p)
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
		log.Printf("Checking if %s matches", p)
		if ignorePaths.Matches(finalPath) {
			if i.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		log.Printf("%s does match", p)
		if !i.IsDir() {
			files[finalPath] = struct{}{}
		}
		return nil
	}
}

func expandPaths(log *log.Logger, templateMap templateFinder, paths []string) ([]string, error) {
	// ignorePaths filenameMatcher
	files := make(map[string]struct{}, len(paths))
	cb := walkCallback(log, templateMap, files)
	for _, path := range paths {
		if strings.HasSuffix(path, "/...") {
			log.Printf("At %s\n", path)
			if err := filepath.Walk(filepath.Dir(path), cb); err != nil {
				return nil, err
			}
		} else {
			files[filepath.Clean(path)] = struct{}{}
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
	cmd  string
	args []string
	cwd  string
}

func (c *cmdInDir) String() string {
	return fmt.Sprintf("CWD=%s %s %s", c.cwd, c.cmd, strings.Join(c.args, " "))
}

func streamLines(input io.Reader, into chan<- string, wg *sync.WaitGroup) {
	defer wg.Done()
	r := bufio.NewScanner(input)
	for r.Scan() {
		into <- r.Text()
	}
}

func streamInto(log *log.Logger, from <-chan string, into io.Writer, wg *sync.WaitGroup) {
	defer wg.Done()
	for l := range from {
		if l != "" {
			_, err := io.WriteString(into, l)
			logIfError(log, "Unable to write out: %s", err)
			_, err = io.WriteString(into, "\n")
			logIfError(log, "Unable to write string: %s", err)
		}
	}
}

func logIfError(log *log.Logger, msg string, err error) {
	if err != nil {
		log.Printf(msg, err.Error())
	}
}

// Execute the command streaming lines of stdin and stdout.  Blocks until exec() is finished or the
// given context closes.  If the context closes early, it will try to kill the spawned connection.
func (c *cmdInDir) exec(ctx context.Context, log *log.Logger, stdoutStream chan<- string, stderrStream chan<- string) error {
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
			av.Store(err)
			return
		}
	}()
	select {
	case <-ctx.Done():
		logIfError(log, "Error killing process", r.Process.Kill())
		<-doneWaiting
		return ctx.Err()
	case <-doneWaiting:
		if err := av.Load(); err != nil {
			return err.(error)
		}
		return nil
	}
}
