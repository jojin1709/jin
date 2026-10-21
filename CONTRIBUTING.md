# Contributing to Jin

Thank you for wanting to help. Jin is small on purpose, and most of the work
that helps is careful work on a small codebase rather than new features.

This file is the long version. The short version is in
[AGENTS.md](AGENTS.md), which is what an AI coding agent should read first.

## Getting set up

You need **Go 1.21 or newer** and nothing else. Jin has no third-party
dependencies, so a fresh checkout builds offline.

```sh
git clone https://github.com/jojin1709/jin
cd jin
go build -o jin.exe .
go test ./...
```

The suite runs in a few seconds. If it does not, something is wrong before you
start.

## Before you commit

```sh
gofmt -l .            # must print nothing
go vet ./...          # must pass
go test -count=1 ./... # must pass
```

If you touched the website:

```sh
python site/build.py            # rebuild the pages
python site/check-site.py       # every link and asset must exist
python site/check-examples.py   # every documented example must run
```

`site/check-examples.py` runs every program on every documentation page with
the real `jin` binary and compares what it prints with what the page says it
prints. A page whose example has drifted fails the check.

`docs/` is generated output. Never edit it by hand: change the body fragment
in `site/pages/` or the template in `site/templates/`, then rebuild.

## The rules that matter

### Two engines, one meaning

Jin has a bytecode VM (`vm.go`) and a tree-walking interpreter
(`interpreter.go`). Every program has to produce **byte-identical output and
error text** on both, and the test suite enforces it.

If you add a language feature, the feature needs differential tests that run
both engines and compare. If the two engines disagree, one of them has a bug,
and the test has to say which.

### Shared logic stays in one place

Anything both engines need has to exist once: the built-ins in `builtins.go`,
the record and field handling in `record.go`, and the indexed reads and writes
in `vm.go`. If you find yourself writing the same rule twice, put it in the
shared place instead.

### The type checker runs before the program does

Jin's promise is that mistakes are found before the program starts. If you
change what the checker accepts, or what a message says, change the tests and
the documentation together. A new error message is part of the product.

### No third-party dependencies

Pure Go, standard library only. `go.mod` stays on `go 1.21`, so do not use
language features or standard-library APIs that are newer.

## Pull requests

- One change per pull request. A change that does two things is two pull
  requests.
- Say what the change does and why in the description. If it fixes an issue,
  say which one.
- Keep the commit history readable: a clear message, and a body that explains
  the reason rather than repeating the diff.
- Small fixes are welcome as pull requests. For something bigger, open an
  issue first so the shape of it can be agreed before you write it.

## Commit messages

A short line that says what changed, in the imperative:

```
fix: a string inside an interpolation ends the string it sits in
type checker: a name that starts as nil is not pinned to nil
site: fix the hero, the highlighter and the tables
```

## Reporting a bug

Open an issue with:

- the Jin program, small enough to read,
- what you expected it to print,
- what it printed instead,
- the version, from `jin --version`.

If it is a crash or a wrong answer, a failing test in the pull request that
fixes it is the best possible bug report.

## Code of conduct

Everyone taking part is expected to follow the
[Code of Conduct](CODE_OF_CONDUCT.md).

## Licence

Jin is available under the MIT licence ([LICENSE](LICENSE)) or the Apache
Licence 2.0 ([LICENSE-APACHE](LICENSE-APACHE)), at your option. A
contribution is accepted under the same terms.
