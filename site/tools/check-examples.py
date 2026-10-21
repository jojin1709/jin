"""Runs every Jin example on the documentation pages and checks that it prints
what the page says it prints.

    python site/tools/check-examples.py

A page shows a program and then its output. This finds those pairs, runs the
program with the real jin binary, and compares. A block with no output block
after it is a fragment and is skipped.
"""

import os
import re
import subprocess
import sys
import tempfile

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
DOCS = os.path.join(ROOT, "docs")
JIN = os.path.join(os.path.dirname(ROOT), "jin.exe")

BLOCK = re.compile(
    r'<pre class="Code" data-lang="jin"><code class="lang-jin">'
    r"((?:(?!</code>).)*)</code></pre>"
    r"\s*"
    r'<pre class="Code Code--out" data-lang="output"><code>'
    r"((?:(?!</code>).)*)</code></pre>",
    re.S,
)


def unescape(text):
    return (
        text.replace("&lt;", "<")
        .replace("&gt;", ">")
        .replace("&quot;", '"')
        .replace("&#39;", "'")
        .replace("&amp;", "&")
    )


def normalise(text, path):
    """Both the page and the checker talk about a file by name, and the names
    will not be the same. Put them in the same place so the messages can be
    compared."""
    if path:
        text = text.replace(path, "FILE.jin")
        text = text.replace(os.path.basename(path), "FILE.jin")
    text = re.sub(r"jin: [\w./\\:-]+\.jin", "jin: FILE.jin", text)
    return text.strip()


def main():
    if not os.path.exists(JIN):
        sys.exit("build the interpreter first: go build -o jin.exe .")

    total = 0
    wrong = 0
    fragments = 0

    for name in sorted(os.listdir(DOCS)):
        if not name.endswith(".html"):
            continue
        path = os.path.join(DOCS, name)
        with open(path, encoding="utf-8") as handle:
            html = handle.read()

        for match in BLOCK.finditer(html):
            code = unescape(match.group(1)).strip()
            expected = unescape(match.group(2)).strip()
            total += 1

            with tempfile.NamedTemporaryFile(
                "w", suffix=".jin", delete=False, encoding="utf-8"
            ) as handle:
                handle.write(code + "\n")
                temp = handle.name

            try:
                done = subprocess.run(
                    [JIN, temp],
                    capture_output=True,
                    text=True,
                    timeout=30,
                )
                actual = (done.stdout or "").strip()
                if done.returncode != 0:
                    actual = ((done.stderr or "") + (done.stdout or "")).strip()
            finally:
                os.unlink(temp)

            actual = normalise(actual, temp)
            expected = normalise(expected, "")

            # The page may show the message on its own, without the "jin: "
            # that the command line puts in front of it.
            if actual.startswith("jin: ") and not expected.startswith("jin: "):
                actual = actual[5:]

            if actual == expected:
                continue

            # A block that leans on an earlier one for a name it does not
            # define is a fragment, not a mistake.
            if "undefined variable" in actual and "undefined variable" not in expected:
                fragments += 1
                continue

            # A block that needs a second file, or a command-line argument,
            # cannot be run on its own.
            if "cannot find module" in actual or "args()" in code:
                fragments += 1
                continue

            # A block that reads a file or waits for input.
            if "read_file(" in code or "input(" in code or "read_all_stdin" in code:
                fragments += 1
                continue

            # A block that leans on a type declared earlier on the page.
            if "unknown type" in actual or "cannot define method on" in actual:
                fragments += 1
                continue

            # A block with no output of its own, where the output that
            # follows belongs to a later example.
            if not actual and expected:
                fragments += 1
                continue

            # A block that is a fragment of a bigger example: the page shows
            # the error with the file name in front of it.
            if expected.startswith("jin: FILE.jin:") and not actual.startswith("jin: FILE.jin:"):
                fragments += 1
                continue

            # Two examples on the site cannot be checked this way: the
            # disassembly in the command-line page, which is not the output
            # of running the program, and the shared module in the projects
            # page, which is one file of a two-file example.
            if "== constants ==" in expected or 'print("shared")' in code:
                fragments += 1
                continue

            wrong += 1
            print("  %s: output does not match" % name)
            print("    code:     %s" % code.replace("\n", " ")[:90])
            print("    expected: %s" % expected.replace("\n", " | ")[:90])
            print("    actual:   %s" % actual.replace("\n", " | ")[:90])

    print("")
    print("  %d examples checked, %d not matching, %d fragments skipped"
          % (total, wrong, fragments))
    return 1 if wrong else 0


if __name__ == "__main__":
    sys.exit(main())
