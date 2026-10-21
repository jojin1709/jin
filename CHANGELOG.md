# Changelog

All notable changes to Jin are written down here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and Jin uses
[semantic versioning](https://semver.org/spec/v2.0.0.html).

## [0.10.1] — 2026-10-21

### Fixed
- A string inside an `{interpolation}` no longer ends the string it sits in, so
  `print("fib: {join(fibs, ", ")}")` runs. Both the plain and the escaped form
  work.
- A file that begins with a byte order mark is no longer a syntax error. The
  mark is skipped, so a file saved by a Windows editor runs.
- A block that is never closed is reported as `missing '}' at the end of the
  file`, instead of quietly swallowing the rest of the file.
- `jin check` now catches an index that is written out in full, so
  `[1, 2, 3][9]` is reported before the program runs. An index that is not
  written out in full stays a run-time error.
- A name that starts as `nil` is no longer pinned to `nil`. It is left open,
  and takes whatever it is given later, the way an empty array or an empty map
  already worked.

### Added
- The version is a variable, so a release build can stamp it with
  `-ldflags "-X main.version=..."`. A build from source says `dev`.
- The module is `github.com/jojin1709/jin`, so `go install` works.
- The website: a landing page, a browser playground that runs real Jin, and
  the full documentation.
- A code of conduct and a contributing guide.
- The project is available under the MIT licence or the Apache Licence 2.0, at
  your option.

### Changed
- The README and the website no longer claim Jin is much faster than Python.
  The measured claim is the true one: on naive `fib(30)` the VM and CPython
  take about the same time.

## [0.10.0] — 2026-10-19

### Added
- Terminal input: `input()` with an optional prompt, and `read_all_stdin()`.
- Multi-line strings with `"""`, with string interpolation inside them.
- Higher-order built-ins: `map`, `filter`, `reduce`, `any`, `all`, `sort_by`
  and `find_index`.
- Named record construction: `Point{x: 1, y: 2}`.
- `read_file`, `write_file`, `append_file`, `file_exists` and `delete_file`.

### Changed
- A hardening pass over callbacks, input, multi-line strings, the parser and
  the error messages.

## [0.9.0] — 2026-10-19

### Added
- `try` and `catch` statements, and the `error()` built-in for raising an
  error with your own message.
- Catchable runtime errors: failed file operations, out-of-bounds array
  indices, missing map keys, division by zero, and record fields that are not
  there.
- Re-raising an error from a catch block, keeping the position it came from.

### Changed
- The README was brought in line with the code, and a CI workflow was added.

## [0.8.0] — 2026-10-19

### Added
- Records: user-defined struct types, declared with `struct`, constructed
  positionally or by name.
- Field access and field mutation, with `p.x = 3`.
- Methods, declared in receiver syntax: `fn (p: Point) shift(dx: int) -> Point`.
- Bound methods as ordinary first-class values: `let move = p.shift`.

## [0.7.0] — 2026-10-19

### Added
- Imports: `import "utils" as u` reaches another file's top-level names
  through an alias.
- Projects: a folder holding a `jin.toml` is a project, and `jin new` starts
  one.
- The command line: `run`, `interpret`, `check`, `build`, `new`, `compile`,
  `parse` and `tokens`.
- `args()`, so a program can read the arguments it was run with.
