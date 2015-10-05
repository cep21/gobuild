package main

import (
	"testing"
	"bytes"
	"github.com/BurntSushi/toml"
	. "github.com/smartystreets/goconvey/convey"
	"golang.org/x/net/context"
	"time"
	"os/exec"
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
	Convey("When executing something on multiple lines", t, func(c2 C) {
		c := cmdInDir{
			cmd: "echo",
			args: []string{"hello\nworld"},
			cwd: ".",
		}
		ctx := context.Background()
		stdoutStream := make(chan string)
		execDone := make(chan struct{})
		go func() {
			c2.So(c.exec(ctx, stdoutStream, nil), ShouldBeNil)
			close(execDone)
		}()
		Convey("Should be able to stream from stdout", func() {
			So(<- stdoutStream, ShouldEqual, "hello")
			So(<- stdoutStream, ShouldEqual, "world")
			<- execDone
		})
	})
}

func TestExecNormal(t *testing.T) {
	Convey("When executing something", t, func(c2 C) {
		c := cmdInDir{
			cmd: "echo",
			args: []string{"hello", "world"},
			cwd: ".",
		}
		ctx := context.Background()
		stdoutStream := make(chan string)
		execDone := make(chan struct{})
		go func() {
			c2.So(c.exec(ctx, stdoutStream, nil), ShouldBeNil)
			close(execDone)
		}()
		Convey("Should be able to stream from stdout", func() {
			line := <- stdoutStream
			So(line, ShouldEqual, "hello world")
			<- execDone
		})
	})
}

func TestExecInvalid(t *testing.T) {
	Convey("When executing something that is invalid", t, func(c2 C) {
		c := cmdInDir{
			cmd: "asdfdsafasd",
			cwd: ".",
		}
		ctx := context.Background()
		Convey("We should read an error", func() {
			So(c.exec(ctx, nil, nil).(*exec.Error).Err, ShouldEqual, exec.ErrNotFound)
		})
	})
}

func TestExecTimeout(t *testing.T) {
	Convey("When executing something that times out", t, func(c2 C) {
		c := cmdInDir{
			cmd: "sleep",
			args: []string{"200"},
			cwd: ".",
		}
		ctx, _ := context.WithTimeout(context.Background(), time.Millisecond * 10)
		Convey("We should read the timeout", func() {
			So(c.exec(ctx, nil, nil), ShouldEqual, ctx.Err())
		})
	})
}
