# The Jin website

The static site for the Jin programming language: a landing page, a browser
playground, and the full documentation.

It is plain HTML, CSS and JavaScript. There is no framework, no bundler and
no third-party runtime — nothing to install and nothing to keep up to date.

## Layout

```
site/
  index.html            the landing page          (generated)
  docs/                 the documentation         (generated)
    index.html          the documentation hub
    tutorial.html       getting started
    tour.html           the language tour
    types.html          the type system
    records.html        records and methods
    errors.html         error handling
    builtins.html       the standard library
    modules.html        projects and imports
    cli.html            the command line
    architecture.html   how it all works
    roadmap.html        status and roadmap
  assets/
    css/site.css        the whole stylesheet
    js/site.js          theme, nav, code blocks, the playground
    js/jin.js           a Jin interpreter, for the playground
    js/examples.js      the playground's example programs
    img/                the logo, the icon and the banner
  tools/
    build.py            assembles the site
    templates/base.html the shared layout
    pages/              one body fragment per page
    test-interpreter.js runs the playground examples
```

## Building

```sh
python site/tools/build.py            # regenerate the HTML from the fragments
python site/tools/check-site.py       # every link and asset must exist
python site/tools/check-examples.py   # every documented example must run
node site/tools/test-interpreter.js   # run the playground examples
```

`build.py` reads `tools/templates/base.html` and the body fragments in
`tools/pages/`, and writes `index.html` and `docs/*.html`. The header, the
navigation, the footer and the documentation sidebar are written once, in the
template and in `build.py`, instead of twelve times.

`check-site.py` walks the built pages and makes sure every local link and
every asset points at something that exists, and that every page has a title,
a description and a main landmark.

`check-examples.py` finds every program on the documentation pages, runs it
with the real `jin` binary, and compares what it prints with the output the
page says it prints. It skips the examples that need a second file, a
command-line argument, or a name from an earlier block on the page.

## Publishing

`.github/workflows/pages.yml` publishes the `site/` directory to GitHub Pages
on every push to `main` that touches it. Turn it on once:

**Settings → Pages → Build and deployment → Source: GitHub Actions**

The site then lives at `https://jojin1709.github.io/jin/`.

## The playground

`assets/js/jin.js` is a tree-walking interpreter for a subset of Jin, written
in JavaScript so the landing page can run real code in the browser. It covers
values, control flow, functions, closures, arrays, maps, records, string
interpolation, `try`/`catch` and the built-ins.

It is not the whole language: imports, file access and terminal input are not
available in a browser, and the interpreter says so rather than guessing. Every
example in `assets/js/examples.js` is run against both this interpreter and the
real `jin` binary, and the two have to produce the same output.
