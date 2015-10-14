package main

import (
	"bytes"
	"io/ioutil"
	"log"
	"os/exec"
	"testing"
	"time"

	"github.com/cep21/gobuild/internal/github.com/BurntSushi/toml"
	. "github.com/cep21/gobuild/internal/github.com/smartystreets/goconvey/convey"
	"github.com/cep21/gobuild/internal/golang.org/x/net/context"
)

func TestLoadMacro(t *testing.T) {
	Convey("When loading macros", t, func() {
		g := gobuildInfo{}
		r := bytes.NewBufferString(defaultTemplate)
		_, err := toml.DecodeReader(r, &g)
		So(err, ShouldBeNil)
		Convey("Var defaults should work", func() {
			So(g.Vars["duplthreshold"], ShouldEqual, 75)
		})
	})
}

func TestExecMultiLine(t *testing.T) {
	Convey("When executing something on multiple lines", t, func() {
		c := cmdInDir{
			cmd:  "echo",
			args: []string{"hello\nworld"},
			cwd:  ".",
		}
		ctx := context.Background()
		stdoutStream := make(chan string, 2)
		l := log.New(ioutil.Discard, "", 0)
		Convey("Should be able to stream from stdout", func(c2 C) {
			execWait := make(chan error)
			go func() {
				execWait <- c.exec(ctx, l, stdoutStream, nil)
			}()
			So(<-stdoutStream, ShouldEqual, "hello")
			So(<-stdoutStream, ShouldEqual, "world")
			So(<-execWait, ShouldBeNil)
		})
	})
}

func TestExecNormal(t *testing.T) {
	Convey("When executing something", t, func(c2 C) {
		c := cmdInDir{
			cmd:  "echo",
			args: []string{"hello", "world"},
			cwd:  ".",
		}
		ctx := context.Background()
		stdoutStream := make(chan string)
		execDone := make(chan error)
		go func() {
			l := log.New(ioutil.Discard, "", 0)
			execDone <- c.exec(ctx, l, stdoutStream, nil)
		}()
		Convey("Should be able to stream from stdout", func() {
			line := <-stdoutStream
			So(line, ShouldEqual, "hello world")
			So(<-execDone, ShouldBeNil)
		})
	})
}

func TestExecInvalid(t *testing.T) {
	Convey("When executing something that is invalid", t, func() {
		c := cmdInDir{
			cmd: "asdfdsafasd",
			cwd: ".",
		}
		ctx := context.Background()
		Convey("We should read an error", func() {
			l := log.New(ioutil.Discard, "", 0)
			So(c.exec(ctx, l, nil, nil).(*exec.Error).Err, ShouldEqual, exec.ErrNotFound)
		})
	})
}

func TestExecTimeout(t *testing.T) {
	Convey("When executing something that times out", t, func(c2 C) {
		c := cmdInDir{
			cmd:  "sleep",
			args: []string{"200"},
			cwd:  ".",
		}
		ctx, _ := context.WithTimeout(context.Background(), time.Millisecond*10)
		Convey("We should read the timeout", func() {
			l := log.New(ioutil.Discard, "", 0)
			So(c.exec(ctx, l, nil, nil), ShouldEqual, ctx.Err())
		})
	})
}
