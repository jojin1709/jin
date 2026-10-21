# Page body conventions

Every `*.body.html` file is a **fragment** — no `<html>`, `<head>`, `<body>`,
no header, no footer, no sidebar. The build script (`site/tools/build.py`)
wraps it in `site/tools/templates/base.html` and adds the header, footer and
the docs sidebar.

The fragment is the **content of `<main>` only**.

## Ground rules

- Facts must come from the actual implementation in `D:\Projects\jin` — the Go
  source, the tests and `examples/`. The README is a good starting point but
  the code is the truth. If they disagree, the code wins and the page should
  say what the code does.
- No placeholder text, no "TODO", no lorem ipsum. Every page is complete.
- Write in clear, plain English. Short sentences. No marketing filler.
- Escape `<` as `&lt;` and `>` as `&gt;` inside code blocks. Everything else
  is literal HTML.
- Jin code inside `<code>` blocks: use `&lt;` for `<` and `&gt;` for `>`.
  `{` and `}` are fine literal in HTML text.

## Allowed HTML

```html
<!-- a section with an id, for the sidebar and deep links -->
<section id="slug">
  <h2>Section title</h2>
  <p>Prose. <code>inline code</code> and <a href="...">links</a> work.</p>

  <h3>Subsection</h3>
  <p>Prose.</p>

  <pre class="Code" data-lang="jin"><code class="lang-jin">fn fib(n) {
    if n &lt; 2 { return n }
    return fib(n - 1) + fib(n - 2)
}</code></pre>

  <pre class="Code" data-lang="shell"><code class="lang-shell">go build -o jin.exe .
./jin.exe examples/tour.jin</code></pre>

  <pre class="Code Code--out" data-lang="output"><code>fib: 0, 1, 1, 2, 3, 5, 8, 13, 21, 34</code></pre>

  <table class="Table">
    <thead>
      <tr><th>Name</th><th>Signature</th><th>What it does</th></tr>
    </thead>
    <tbody>
      <tr><td><code>len</code></td><td><code>len(x)</code></td><td>The length of a string, array or map.</td></tr>
    </tbody>
  </table>

  <aside class="Callout Callout--note">
    <p><strong>Note.</strong> Something worth knowing.</p>
  </aside>

  <aside class="Callout Callout--warning">
    <p><strong>Careful.</strong> Something that will bite.</p>
  </aside>

  <aside class="Callout Callout--error">
    <p><strong>Error.</strong> Something that fails.</p>
  </aside>

  <ul class="Bullets">
    <li>A point.</li>
  </ul>

  <ol class="Steps">
    <li>First step.</li>
  </ol>
</section>
```

`data-lang` on `<pre>` drives the label in the corner and the highlighting:
`jin`, `shell`, `output`, `toml`, `text`. `lang-*` on the inner `<code>` is
what the highlighter keys off; use `lang-jin`, `lang-shell`, `lang-text`.

## Page-specific requirements

Every documentation page starts with:

```html
<div class="DocHeader">
  <p class="DocHeader-crumb"><a href="index.html">Docs</a> / Page name</p>
  <h1>Page name</h1>
  <p class="DocHeader-lede">One or two sentences on what this page covers.</p>
</div>

<div class="DocLayout">
  <div class="DocMain">
    ...sections...
  </div>
</div>
```

The build script inserts the sidebar *outside* `.DocMain`, so do not write it.

## Definitions of done

1. Every construct documented has a runnable example.
2. Every example has been run against the real interpreter and the output in
   the page is the output it actually produced.
3. Every table row is accurate against the source.
4. Every page ends with a "Next" link to the following page.
5. No link in the page points at a file that does not exist.
