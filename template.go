package main
import (
	"strings"
	"path/filepath"
	"sort"
	"os"
)

type buildTemplate struct {
	Install install `toml:"install"`
	Vars map[string]interface{} `toml:"vars"`
}

type install struct {
	Goget map[string]string `toml:"goget"`
}

var defaultMetalinterArgs = []string{"-t", "--disable-all"}

func (b *buildTemplate) MetalintArgs() []string {
	return defaultMetalinterArgs
}

func (b *buildTemplate) BuildFlags() []string {
	return []string{}
}

func (b *buildTemplate) TestCoverageArgs() []string {
	return nil
}

func (b *buildTemplate) MetalintIgnoreLines() []string {
	return []string{}
}

func (b *buildTemplate) DuplArgs() []string {
	return []string{"-files", "-t", "100", "-plumbing"}
}

func (b *buildTemplate) IgnoreDirs() []string {
	ignores, exists := b.Vars["IgnoreDirs"]
	if !exists {
		return []string{}
	}
	ignoresArr, ok := ignores.([]string)
	if !ok {
		return []string{}
	}
	return ignoresArr
}


type templateCache struct {
	cache map[string]*buildTemplate
}

func (t *templateCache) loadInDir(dir string) (*buildTemplate, error) {
	return nil, nil
}

type pathExpansion struct {
	forceAbs bool
	log logger
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
		p.log.Printf("At %s\n", path)
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

func (p *pathExpansion) expandPaths(paths []string) {
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
