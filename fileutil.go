package main

import (
	"path/filepath"
	"sort"
)

// filesWithGlobInDir returns all the files in dirs that match glob
func filesWithGlobInDir(dirs []string, glob string) ([]string, error) {
	if glob == "" {
		return dirs, nil
	}
	ret := make([]string, 0, len(dirs))
	for _, d := range dirs {
		matches, err := filepath.Glob(filepath.Join(d, glob))
		if err != nil {
			return nil, wraperr(err, "glob matching failed")
		}
		ret = append(ret, matches...)
	}
	sort.Strings(ret)
	return ret, nil
}

// dirsWithFileGob filters dirs to only include files with glob string inside
func dirsWithFileGob(dirs []string, glob string) ([]string, error) {
	if glob == "" {
		return dirs, nil
	}
	matchedFiles, err := filesWithGlobInDir(dirs, glob)
	if err != nil {
		return nil, wraperr(err, "subcall to filesWithGlobInDir failed")
	}
	ret := make([]string, 0, len(dirs))
	matchedFilesMap := make(map[string]struct{}, len(matchedFiles))
	for _, f := range matchedFiles {
		matchedFilesMap[filepath.Dir(f)] = struct{}{}
	}
	for f := range matchedFilesMap {
		ret = append(ret, f)
	}
	sort.Strings(ret)
	return ret, nil
}
