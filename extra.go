package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"

	"path/filepath"
)

type logger interface {
	Printf(string, ...interface{})
}

type wrappedError struct {
	err error
	msg string
}

type multiErrStr struct {
	errs []error
}

func logIfErr(err error, l logger, msg string, args ...interface{}) {
	if err != nil {
		l.Printf(msg, err.Error()+"|"+fmt.Sprintf(msg, args...))
	}
}

func (e *multiErrStr) Error() string {
	r := make([]string, 0, len(e.errs))
	for _, err := range e.errs {
		r = append(r, err.Error())
	}
	return strings.Join(r, " | ")
}

func multiErr(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	retErrs := make([]error, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			retErrs = append(retErrs, err)
		}
	}
	if len(retErrs) == 0 {
		return nil
	}
	return &multiErrStr{
		errs: errs,
	}
}

func wraperr(err error, msg string, args ...interface{}) *wrappedError {
	return &wrappedError{
		err: err,
		msg: fmt.Sprintf(msg, args...),
	}
}

func (e *wrappedError) Error() string {
	return fmt.Sprintf("%s: %s", e.msg, e.err.Error())
}

type nopCloseWriter struct {
	io.Writer
}

func (n *nopCloseWriter) Close() error {
	return nil
}

type myselfOutput struct {
	w io.WriteCloser
}

func (m myselfOutput) GetCmdOutput(cmdName string) (io.WriteCloser, error) {
	return m.w, nil
}

type fileStreamer struct {
	defaultVars      map[string]interface{}
	filenameTemplate *template.Template
}

func mergeMap(left, right map[string]interface{}) map[string]interface{} {
	ret := make(map[string]interface{}, len(left)+len(right))
	for _, m := range []map[string]interface{}{left, right} {
		for k, v := range m {
			ret[k] = v
		}
	}
	return ret
}

func (d *fileStreamer) GetCmdOutput(cmdName string) (io.WriteCloser, error) {
	fileName := bytes.Buffer{}

	if err := d.filenameTemplate.Execute(&fileName, mergeMap(d.defaultVars, map[string]interface{}{
		"cmdName": cmdName,
	})); err != nil {
		return nil, wraperr(err, "unable to generate template for cmd %s", cmdName)
	}

	f, err := os.Create(fileName.String())
	if err != nil {
		return nil, wraperr(err, "cannot create file %s", fileName.String())
	}
	return f, nil
}

func inDirStreamer(dir string) *fileStreamer {
	defaultVars := map[string]interface{}{
		"dir": dir,
	}
	funcMap := template.FuncMap{
		"CreateName": func(dir string, cmd string) string {
			return filepath.Join(dir, sanitizeFilename(cmd))
		},
	}
	ft := template.Must(template.New("for dir").Funcs(funcMap).Parse("{{ CreateName .dir .cmdName }}"))
	return &fileStreamer{
		defaultVars:      defaultVars,
		filenameTemplate: ft,
	}
}

type cmdOutputStreamer interface {
	GetCmdOutput(cmdName string) (io.WriteCloser, error)
}

func panicIfNotNil(err error, msg string, args ...interface{}) {
	if err != nil {
		fmt.Fprintf(os.Stderr, msg+"\n", args...)
		panic(err)
	}
}
