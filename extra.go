package main
import (
	"fmt"
	"strings"
	"os/exec"
"golang.org/x/net/context"
	"io"
	"os"
	"path/filepath"
	"text/template"
	"bytes"
)

type logger interface {
	Printf(string, ...interface{})
}

type wrappedError struct {
	err error
	msg string
}

type multiErr struct {
	errs []error
}

func logIfErr(err error, l logger, msg string, args ... interface{}) {
	if err != nil {
		l.Printf(msg, err.Error() + "|" + fmt.Sprintf(msg, args...))
	}
}

func (e *multiErr) Error() string {
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
	return &multiErr{
		errs: errs,
	}
}

func wraperr(err error, msg string, args ...interface{}) *wrappedError {
	return wrappedError{
		err: err,
		msg: fmt.Sprintf(msg, args...),
	}
}

func (e *wrappedError) Error() string {
	return fmt.Sprintf("%s: %s", e.msg, e.err.Error())
}

func runInContext(ctx context.Context, cmd *exec.Cmd, verboseLog logger, warningLog logger) error {
	if err := cmd.Start(); err != nil {
		return wraperr(err, "uanble to start command")
	}

	doneWaiting := make(chan struct{})
	//	av := atomic.Value{}
	var waitError error
	go func() {
		defer close(doneWaiting)
		waitError = cmd.Wait()
	}()
	select {
	case <-ctx.Done():
		logIfError(warningLog, "Error killing process", cmd.Process.Kill())
		// The above kill should cause cmd.Wait() to finish
		<-doneWaiting
		return ctx.Err()
	case <-doneWaiting:
		return waitError
	}
}

type cmdOutput interface {
	Stdout() io.Writer
	Stderr() io.Writer
	io.Closer
}

type myselfOutput struct{}

func (m myselfOutput) GetCmdOutput(cmdName string) cmdOutput {
	return m
}

func (m myselfOutput) Stdout() io.Writer {
	return os.Stdout
}

func (m myselfOutput) Stderr() io.Writer {
	return os.Stderr
}

func (m myselfOutput) Close() error {
	return nil
}

type streamOutput struct {
	stdout io.WriteCloser
	stderr io.WriteCloser
}

func (m *streamOutput) Stdout() io.Writer {
	return m.stdout
}

func (m *streamOutput) Stderr() io.Writer {
	return m.stderr
}

func (m *streamOutput) Close() error {
	errs := make([]error, 0, 2)
	errs = append(errs, m.stdout.Close())
	if m.stderr != m.stdout {
		errs = append(errs, m.stderr.Close())
	}
	// Don't let people access it again after closing
	m.stdout = nil
	m.stderr = nil
	return multiErr(errs)
}

type filenameTemplate struct {
	rootDir string
	filenameCreator template.Template
}

func (d *filenameTemplate) GetCmdOutput(cmdName string) (cmdOutput, error) {
	fileNames := []bytes.Buffer{{bytes.Buffer{}, bytes.Buffer{}}}

	for idx, stream := range []string{"stdout", "stderr"} {
		if err := d.filenameCreator.Execute(&fileNames[idx], map[string]string{
			"stream": stream,
			"cmdName": cmdName,
			"root": d.rootDir,
		}); err != nil {
			return wraperr(err, "unable to generate template for cmd %s stream %s", cmdName, stream)
		}
	}
	stdoutFile, err := os.Create(fileNames[0])
	if err != nil {
		return nil, wraperr(err, "cannot create file %s", fileNames[0])
	}
	if fileNames[0] == fileNames[1] {
		return &streamOutput{
			stdout: stdoutFile,
			stderr: stdoutFile,
		}, nil
	}
	stderrFile, err := os.Create(fileNames[1])
	if err != nil {
		return nil, wraperr(err, "cannot create file %s", fileNames[1])
	}
	return &streamOutput{
		stdout: stdoutFile,
		stderr: stderrFile,
	}, nil
}

type cmdOutputStreamer interface {
	GetCmdOutput(cmdName string) (cmdOutput, error)
}

