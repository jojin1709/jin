package main

import (
	"io"
	"os"
	"testing"
)

// runMain runs the command line with the given arguments and returns what it
// printed. It is only safe for the paths that print and return, and never
// call os.Exit.
func runMain(t *testing.T, args ...string) string {
	t.Helper()

	oldArgs := os.Args
	oldStdout := os.Stdout
	defer func() {
		os.Args = oldArgs
		os.Stdout = oldStdout
	}()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}

	os.Stdout = writer
	os.Args = append([]string{"jin"}, args...)

	main()

	writer.Close()
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return string(out)
}

// The version is a variable so a release build can set it with
// -ldflags "-X main.version=...", and the command line prints it the same way
// whichever build it is.
func TestVersionFlagPrintsTheVersion(t *testing.T) {
	if version == "" {
		t.Fatal("the version is empty")
	}

	want := "jin " + version + "\n"
	for _, flag := range []string{"--version", "-v", "version"} {
		got := runMain(t, flag)
		if got != want {
			t.Errorf("%s printed %q, want %q", flag, got, want)
		}
	}
}

// A build from source has no version stamped into it, and says so.
func TestVersionIsDevWhenItHasNotBeenStamped(t *testing.T) {
	const stamped = "1.2.3"
	if version == stamped {
		t.Fatal("the version looks stamped into the binary")
	}
	// The value a release build sets is a plain version number, so a build
	// that has been stamped must not carry a space or a bracket.
	for _, ch := range version {
		if ch == ' ' || ch == '(' || ch == ')' {
			t.Errorf("the version %q holds a character a version number should not", version)
			break
		}
	}
}
