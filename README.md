# gobuild

Code lint and build tool for go code on build servers and developer machines.

# Features

* Runs in windows/linux/mac without extra dependencies
* Simple usage with minimal command line parameters
* Can run in subdirectories with understood usage of operating on sub-subdirectories
* Parallelism of checks
* Auto download of dependencies
* Easy to read config format
* Can auto format code for developers
* Easy to understand error messages
* Optional verbose output
* Inheritance of usage via directory structure
* Understands limiting directory search by git project
* Can ignore directories, files, or messages
* Easy to templatize
* Builder versioning
* Supports --diff option understanding recent changes from git release branch.

# Expected usage ...

## ... as a developer ...

### ... to check everything

```
gobuild -fix
``` 

### ... to run a faster check from origin/master

```
gobuild -fix -diff
```

## ... as a build system

```
gobuild
```

# Why not just use ...

## ... go build

I want to automate running of gofmt, go vet, and other checks.  There are also multiple binaries
that can be built.

## ... bash scripts

I want easy windows support without installing cygwin and other windows dependencies.  I also want
to templatize the common parts.  Finally, I subjectively think go is a better cross platform
language than bash.

## ... gometalinter

I want subdirectory reconfiguration and auto formatting.  Also, the simplicity of a running the
command without any arguments and it doing the right thing was attractive.

## ... cmake

Cmake on windows requires a bit more setup than I want.  Also sharing templates was more work than
I wanted to push on users

## ... maven

Configuration format (xml) is a bit difficult to read.
