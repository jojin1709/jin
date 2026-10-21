# The Jin website

The site for the Jin programming language: a landing page, a browser
playground, and the full documentation.

It is plain HTML, CSS and JavaScript. There is no framework, no bundler and
no third-party runtime — nothing to install and nothing to keep up to date.

## Layout

```
docs/                   the published site
  index.html            the landing page
  docs/                 the documentation
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
  .nojekyll             tells GitHub Pages to serve the files as they are

site/                   the source of the site
  build.py              assembles the site
  templates/base.html   the shared layout
  pages/                one body fragment per page
  check-site.py         every link and asset must exist
  check-examples.py     every documented example must run
  test-interpreter.js   runs the playground examples
```

## Building

```sh
python site/build.py            # regenerate the HTML from the fragments
python site/check-site.py       # every link and asset must exist
python site/check-examples.py   # every documented example must run
node site/test-interpreter.js   # run the playground examples
node site/screenshot.js         # picture every page and look for faults
```

`screenshot.js` needs Playwright, which is not a dependency of the project:
`npm install playwright` in a scratch folder, then point `NODE_PATH` at it.
It opens every page at a desktop and a phone width, saves a picture of each
one, and reports a missing asset, a console error, an image in the wrong
shape, or a page that scrolls sideways.

`build.py` reads `site/templates/base.html` and the body fragments in
`site/pages/`, and writes `docs/index.html` and `docs/docs/*.html`. The
header, the navigation, the footer and the documentation sidebar are written
once, in the template and in `build.py`, instead of twelve times.

`check-site.py` walks the built pages and makes sure every local link and
every asset points at something that exists, and that every page has a title,
a description and a main landmark.

`check-examples.py` finds every program on the documentation pages, runs it
with the real `jin` binary, and compares what it prints with the output the
page says it prints. It skips the examples that need a second file, a
command-line argument, or a name from an earlier block on the page.

## Publishing

The site is published from the `docs/` folder, which is the folder GitHub
Pages expects. In **Settings → Pages → Build and deployment**, set:

- Source: **Deploy from a branch**
- Branch: **main**, folder: **/docs**

`.github/workflows/pages.yml` publishes the same folder through GitHub Actions
instead, if you would rather set the source to **GitHub Actions**. Either way
the site is at `https://jojin1709.github.io/jin/`.

The `.nojekyll` file in `docs/` matters: it tells GitHub Pages to serve the
files as they are, without running them through Jekyll. Without it Jekyll
would try to read the `{` and `{{` in the Jin code samples as its own
template syntax, and the build would fail.

## The playground

`docs/assets/js/jin.js` is a tree-walking interpreter for a subset of Jin,
written in JavaScript so the landing page can run real code in the browser. It
covers values, control flow, functions, closures, arrays, maps, records,
string interpolation, `try`/`catch` and the built-ins.

It is not the whole language: imports, file access and terminal input are not
available in a browser, and the interpreter says so rather than guessing. Every
example in `docs/assets/js/examples.js` is run against both this interpreter
and the real `jin` binary, and the two have to produce the same output.
