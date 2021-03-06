package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cep21/gobuild/internal/github.com/BurntSushi/toml"
)

type buildTemplate struct {
	Install    install                `toml:"install"`
	Metalinter metalinter             `toml:"metalinter"`
	Vars       map[string]interface{} `toml:"vars"`
	Fix        fixes                  `toml:"fix"`
}

type fixes struct {
	Commands map[string]bool `toml:"commands"`
}

func (i *fixes) MergeFrom(from *fixes) {
	if from == nil {
		return
	}
	if len(from.Commands) > 0 && i.Commands == nil {
		i.Commands = make(map[string]bool, len(from.Commands))
	}
	for k, v := range from.Commands {
		i.Commands[k] = v
	}
}

func (b *buildTemplate) FixesEnabled() map[string]bool {
	return b.Fix.Commands
}

type install struct {
	Goget map[string]string `toml:"goget"`
}

func (i *install) MergeFrom(from *install) {
	if from == nil {
		return
	}
	if len(from.Goget) > 0 && i.Goget == nil {
		i.Goget = make(map[string]string, len(from.Goget))
	}
	for k, v := range from.Goget {
		i.Goget[k] = v
	}
}

type metalinter struct {
	Enabled map[string]bool        `toml:"enabled"`
	Ignored map[string]string      `toml:"ignored"`
	Vars    map[string]interface{} `toml:"vars"`
}

func (i *metalinter) MergeFrom(from *metalinter) {
	if from == nil {
		return
	}
	if len(from.Enabled) > 0 && i.Enabled == nil {
		i.Enabled = make(map[string]bool, len(from.Enabled))
	}
	for k, v := range from.Enabled {
		i.Enabled[k] = v
	}

	if len(from.Ignored) > 0 && i.Ignored == nil {
		i.Ignored = make(map[string]string, len(from.Ignored))
	}
	for k, v := range from.Ignored {
		i.Ignored[k] = v
	}
	i.mergeVars(from)
}

func (i *metalinter) mergeVars(from *metalinter) {
	if len(from.Vars) > 0 && i.Vars == nil {
		i.Vars = make(map[string]interface{}, len(from.Enabled))
	}
	for k, v := range from.Vars {
		i.Vars[k] = v
	}
}

func (b *buildTemplate) MetalintArgs() []string {
	defaultMetalinterArgs := varStrArray(b.Metalinter.Vars, "args")
	ret := make([]string, 0, len(defaultMetalinterArgs)+len(b.Metalinter.Enabled))
	ret = append(ret, defaultMetalinterArgs...)
	for linterName, enabled := range b.Metalinter.Enabled {
		if enabled {
			ret = append(ret, "-E", linterName)
		}
	}
	return ret
}

func (b *buildTemplate) MergeFrom(from *buildTemplate) {
	if from == nil {
		return
	}
	b.Install.MergeFrom(&from.Install)
	b.Metalinter.MergeFrom(&from.Metalinter)
	b.Fix.MergeFrom(&from.Fix)
	if len(from.Vars) > 0 && b.Vars == nil {
		b.Vars = make(map[string]interface{}, len(from.Vars))
	}
	for k, v := range from.Vars {
		b.Vars[k] = v
	}
}

func (b *buildTemplate) BuildFlags() []string {
	return b.varStrArray("buildFlags")
}

func (b *buildTemplate) TestCoverageArgs() []string {
	return b.varStrArray("testFlags")
}

func (b *buildTemplate) MetalintIgnoreLines() []string {
	ret := make([]string, 0, len(b.Metalinter.Ignored))
	for _, v := range b.Metalinter.Ignored {
		if v != "" {
			ret = append(ret, v)
		}
	}
	return ret
}

func (b *buildTemplate) DuplArgs() []string {
	return []string{"-files", "-t", b.varStr("duplLimit")}
}

func (b *buildTemplate) IgnoreDirs() []string {
	return b.varStrArray("ignoreDirs")
}

func (b *buildTemplate) StopLoadingParent() []string {
	return b.varStrArray("stopLoadingParent")
}

func (b *buildTemplate) varStr(name string) string {
	return b.Vars[name].(string)
}

func (b *buildTemplate) varFloat(name string) float64 {
	return b.Vars[name].(float64)
}

func varStrArray(vars map[string]interface{}, name string) []string {
	ignores, exists := vars[name]
	if !exists {
		return []string{}
	}
	ignoresArr, ok := ignores.([]interface{})
	if !ok {
		return []string{}
	}
	ret := make([]string, 0, len(ignoresArr))
	for _, a := range ignoresArr {
		ret = append(ret, a.(string))
	}
	return ret
}

func (b *buildTemplate) varStrArray(name string) []string {
	return varStrArray(b.Vars, name)
}

var defaultLoadedTemplate buildTemplate

func init() {
	_, err := toml.Decode(defaultTemplate, &defaultLoadedTemplate)
	panicIfNotNil(err, "cannot load default template")
}

type templateCache struct {
	cache      map[string]*buildTemplate
	verboseLog logger
}

const buildFileName = "gobuild.toml"

func (t *templateCache) curDirTemplate(dir string) (*buildTemplate, error) {
	var currentDirTemplate *buildTemplate
	fullBuildFilePath := filepath.Join(dir, buildFileName)
	t.verboseLog.Printf("Fresh template %s checking file %s", dir, fullBuildFilePath)

	if l, err := os.Stat(fullBuildFilePath); err == nil && !l.IsDir() {
		currentDirTemplate = &buildTemplate{}
		if _, err := toml.DecodeFile(fullBuildFilePath, currentDirTemplate); err != nil {
			return nil, wraperr(err, "invalid toml file at %s", fullBuildFilePath)
		}
		t.verboseLog.Printf("Loaded template for %s is %v", fullBuildFilePath, currentDirTemplate)
	} else if !os.IsNotExist(err) {
		return nil, wraperr(err, "cannot stat buildfile %s", fullBuildFilePath)
	}
	return currentDirTemplate, nil
}

func (t *templateCache) loadInDir(dir string) (*buildTemplate, error) {
	t.verboseLog.Printf("Loading template for %s", dir)
	if dir == "" {
		return &defaultLoadedTemplate, nil
	}
	dir, err := filepath.Abs(filepath.Clean(dir))
	if err != nil {
		return nil, wraperr(err, "cannot get abs path of %s", dir)
	}
	if cache, exists := t.cache[dir]; exists {
		return cache, nil
	}

	currentDirTemplate, err := t.curDirTemplate(dir)
	if err != nil {
		return nil, wraperr(err, "cannot load templtae for current directory")
	}
	parentDirTemplate := &defaultLoadedTemplate
	if t.shouldLoadParent(dir, currentDirTemplate) {
		if parent := filepath.Dir(dir); parent != dir {
			if parentDirTemplate, err = t.loadInDir(parent); err != nil {
				return nil, wraperr(err, "cannot load parent template %s", parent)
			}
		}
	}

	tmp := &buildTemplate{}
	tmp.MergeFrom(parentDirTemplate)
	tmp.MergeFrom(currentDirTemplate)
	t.cache[dir] = tmp
	return tmp, nil
}

func (t *templateCache) shouldLoadParent(dir string, currTemplate *buildTemplate) bool {
	tmp := &buildTemplate{}
	tmp.MergeFrom(&defaultLoadedTemplate)
	if currTemplate != nil {
		tmp.MergeFrom(currTemplate)
	}
	for _, stopCheck := range tmp.StopLoadingParent() {
		if _, err := os.Stat(filepath.Join(dir, stopCheck)); err == nil {
			return false
		}
	}
	return true
}

type pathExpansion struct {
	forceAbs bool
	log      logger
	template *templateCache
}

func (p *pathExpansion) singlePath(path string) string {
	path = filepath.Clean(path)
	if !p.forceAbs && !filepath.IsAbs(path) {
		return filepath.Clean("./" + path)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	symPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return absPath
	}
	return symPath
}

func (p *pathExpansion) matchDir(storeInto map[string]struct{}) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		p.log.Printf("Path expansion at %s\n", path)
		if err != nil {
			return err
		}
		l, err := os.Stat(path)
		if err != nil {
			return err
		}
		if !l.IsDir() {
			return nil
		}
		finalPath := filepath.Clean(path)
		pathDirName := filepath.Dir(path)
		pathFileName := filepath.Base(finalPath)
		template, err := p.template.loadInDir(pathDirName)
		if err != nil {
			return err
		}

		p.log.Printf("Found template %v\n", template)

		p.log.Printf("Ignore for %s is %s parent=%s", path, template.IgnoreDirs(), pathDirName)
		for _, ignore := range template.IgnoreDirs() {
			if ignore == pathFileName {
				return filepath.SkipDir
			}
		}
		storeInto[p.singlePath(path)] = struct{}{}
		return nil
	}
}

func (p *pathExpansion) expandPaths(paths []string) ([]string, error) {
	files := make(map[string]struct{}, len(paths))
	cb := p.matchDir(files)
	for _, path := range paths {
		if strings.HasSuffix(path, "/...") {
			p.log.Printf("At %s\n", path)
			if err := filepath.Walk(filepath.Dir(path), cb); err != nil {
				return nil, err
			}
		} else {
			p.log.Printf("Including path directly: %s", path)
			if l, err := os.Stat(path); err == nil && l.IsDir() {
				files[p.singlePath(path)] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(files))
	for d := range files {
		out = append(out, d)
	}
	sort.Strings(out)
	return out, nil
}
