# Rules for working on Jin

## Project facts
- Jin is a programming language written from scratch in Go. Pure Go, NO third-party dependencies. go.mod stays on go 1.21: do not use newer language features or standard-library APIs.
- Two engines must agree byte for byte: the bytecode VM (vm.go) and the tree-walking interpreter (interpreter.go). Every language feature needs differential tests that run both and compare output AND error text.
- Shared logic stays in one place (builtins.go, record.go, indexInto/setInto) so the engines cannot drift apart.
- Website source is in site/. docs/ is generated output: never edit docs/ by hand. Site build command: `python site/build.py`. Site check commands: `python site/check-site.py`, `python site/check-examples.py` (needs `go build -o jin.exe .` first), `node site/test-interpreter.js`, and `node site/screenshot.js` (needs Playwright installed and a local server on `http://localhost:8767` serving `docs/`).

## Before every commit
- gofmt -l . must print nothing
- go vet ./... must pass
- go test -count=1 ./... must pass
- if the site changed: rebuild it and run the site checks

## Working rules
1. Do only what the task says. If something else needs fixing, list it in the summary instead of doing it.
2. Tick off every numbered step in the final summary. If you skipped one, say which and why. Never write "done" for something you did not run.
3. Never change an existing test's expected output to make it pass. If you think an expectation is wrong, stop and explain. Adding new tests is always fine.
4. Never put a number, benchmark result or claim ("fast", "faster than X") in the README or site unless you measured it in this session. Say how you measured it.
5. The final summary must contain: files changed, the real "ok" line from go test plus the count of passing tests, gofmt and vet results, and the commit hash. Keep it short.
6. One commit per task, with a clear message, then git push origin main.
7. If a step is ambiguous or blocked, stop and ask ONE question instead of guessing.
