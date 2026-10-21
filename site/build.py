"""
Assembles the Jin website from a shared layout plus per-page bodies.

    python site/tools/build.py

Reads   site/tools/templates/base.html
        site/tools/pages/<name>.body.html
Writes  site/index.html
        site/docs/*.html

The site is plain static HTML: the output of this script is what GitHub Pages
serves. The script exists so the header, the nav, the footer and the docs
sidebar are written once instead of twelve times.
"""

import os
import re
import sys
import time

HERE = os.path.dirname(os.path.abspath(__file__))     # site/
ROOT = os.path.dirname(HERE)            # the repository
SITE = os.path.join(ROOT, "docs")       # the published site
TPL = os.path.join(HERE, "templates")
PAGES = os.path.join(HERE, "pages")
OUT_DOCS = os.path.join(SITE, "docs")

SITE_NAME = "Jin"
SITE_TAGLINE = "A small, fast programming language"
REPO = "https://github.com/jojin1709/jin"
SITE_URL = "https://jojin1709.github.io/jin"
VERSION = "0.10.0"

# Every build stamps its assets with a fresh number, so a browser that has
# the last version cached fetches the new one instead of showing a page
# that does not match the code.
BUILD = time.strftime("%Y%m%d%H%M%S")

# ---------------------------------------------------------------- nav model

NAV = [
    {
        "label": "Why Jin",
        "id": "why",
        "items": [
            ("Design rules", "docs/roadmap.html#design-rules",
             "The nine rules the language is built around"),
            ("What is in", "docs/roadmap.html#what-is-in",
             "Everything Jin can do today, honestly listed"),
            ("Architecture", "docs/architecture.html",
             "How a Jin program becomes running bytecode"),
            ("Roadmap", "docs/roadmap.html",
             "What comes after stage 10"),
        ],
    },
    {
        "label": "Learn",
        "id": "learn",
        "items": [
            ("Getting started", "docs/tutorial.html",
             "Install Jin and run your first program"),
            ("Language tour", "docs/tour.html",
             "Every construct in the language, in one page"),
            ("Try Jin", "#playground",
             "Run Jin in your browser, no install needed"),
        ],
    },
    {
        "label": "Docs",
        "id": "docs",
        "items": [
            ("Documentation", "docs/index.html",
             "The index of everything written down"),
            ("Type system", "docs/types.html",
             "Optional annotations, enforced before the program runs"),
            ("Built-ins", "docs/builtins.html",
             "The standard library, function by function"),
            ("Records & methods", "docs/records.html",
             "User-defined types with fields and methods"),
            ("Error handling", "docs/errors.html",
             "try, catch and error()"),
            ("Projects & imports", "docs/modules.html",
             "Splitting a program across files"),
            ("Command line", "docs/cli.html",
             "Every jin subcommand"),
            ("FAQ", "docs/faq.html",
             "The questions that come up, and the short answers"),
        ],
    },
    {
        "label": "Community",
        "id": "community",
        "items": [
            ("GitHub", REPO, "The source, the issues and the releases"),
            ("Source code", REPO + "/tree/main",
             "Lexer, parser, type checker, compiler and VM"),
            ("Examples", REPO + "/tree/main/examples",
             "Runnable programs that ship with Jin"),
            ("Contributing", REPO + "/blob/main/CONTRIBUTING.md",
             "How to set up, and what the tests have to pass"),
            ("Code of conduct", REPO + "/blob/main/CODE_OF_CONDUCT.md",
             "How everyone taking part is expected to behave"),
            ("The author", "https://github.com/jojin1709",
             "JOJIN JOHN, who designed it"),
        ],
    },
]

# ------------------------------------------------------------- docs sidebar

SIDEBAR = [
    ("Overview", [
        ("Documentation", "index.html"),
        ("Getting started", "tutorial.html"),
        ("FAQ", "faq.html"),
    ]),
    ("Language", [
        ("Language tour", "tour.html"),
        ("Type system", "types.html"),
        ("Records & methods", "records.html"),
        ("Error handling", "errors.html"),
    ]),
    ("Reference", [
        ("Built-ins", "builtins.html"),
        ("Command line", "cli.html"),
    ]),
    ("Project", [
        ("Projects & imports", "modules.html"),
        ("Architecture", "architecture.html"),
        ("Status & roadmap", "roadmap.html"),
    ]),
]

# ------------------------------------------------------------------- helpers


def esc(text):
    return (text.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;"))


def sub(template, key, value):
    return template.replace("{{%s}}" % key, value)


def nav_markup(active, home):
    """The desktop nav, from the same model the drawer uses."""
    out = []
    for group in NAV:
        items = []
        for label, href, desc in group["items"]:
            target = href if href.startswith("http") else home + href
            items.append(
                '<li class="Header-submenuItem">'
                '<a href="%s">%s</a>'
                '<p>%s</p>'
                "</li>" % (target, label, desc)
            )
        out.append(
            '<li class="Header-menuItem%s">'
            '<button class="Header-submenuTrigger" aria-expanded="false" '
            'aria-controls="submenu-%s">%s'
            '<svg class="Header-caret" width="12" height="8" viewBox="0 0 12 8" '
            'aria-hidden="true"><path d="M1 1l5 5 5-5" fill="none" '
            'stroke="currentColor" stroke-width="2" stroke-linecap="round"/>'
            "</svg></button>"
            '<ul class="Header-submenu" id="submenu-%s" hidden>%s</ul>'
            "</li>"
            % (
                " Header-menuItem--active" if group["id"] == active else "",
                group["id"],
                group["label"],
                group["id"],
                "".join(items),
            )
        )
    return "".join(out)


def header_markup(active, home):
    groups = []
    for group in NAV:
        items = []
        for label, href, _ in group["items"]:
            target = href if href.startswith("http") else home + href
            items.append('<li><a href="%s">%s</a></li>' % (target, label))
        groups.append(
            '<li><details class="Drawer-group"><summary>%s</summary>'
            '<ul class="Drawer-sublist">%s</ul></details></li>'
            % (group["label"], "".join(items))
        )
    return """  <header class="Header js-header">
    <div class="Header-inner">
      <a class="Header-logo" href="{{HOME}}" aria-label="Jin home">
        <img src="{{ASSETS}}/img/icon.png?v={{V}}" alt="" width="32" height="32">
        <span class="Header-wordmark">Jin</span>
      </a>
      <nav class="Header-nav" aria-label="Main">
        <ul class="Header-menu">%s</ul>
      </nav>
      <div class="Header-actions">
        <a class="Header-github" href="%s" aria-label="Jin on GitHub" title="Jin on GitHub">
          <svg width="20" height="20" viewBox="0 0 16 16" aria-hidden="true">
            <path fill="currentColor" d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27s1.36.09 2 .27c1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0 0 16 8c0-4.42-3.58-8-8-8z"/>
          </svg>
        </a>
        <button class="Header-theme js-theme-toggle" type="button" aria-label="Change colour theme" title="Change colour theme">
          <svg class="Header-themeIcon Header-themeIcon--auto" width="18" height="18" viewBox="0 0 24 24" aria-hidden="true">
            <path fill="currentColor" d="M12 3a9 9 0 1 0 0 18V3z"/>
            <path fill="none" stroke="currentColor" stroke-width="2" d="M12 3a9 9 0 0 1 0 18"/>
          </svg>
          <svg class="Header-themeIcon Header-themeIcon--light" width="18" height="18" viewBox="0 0 24 24" aria-hidden="true">
            <path fill="currentColor" d="M12 17a5 5 0 1 0 0-10 5 5 0 0 0 0 10zm0-13a1 1 0 0 1 1 1v1a1 1 0 1 1-2 0V5a1 1 0 0 1 1-1zm0 15a1 1 0 0 1 1 1v1a1 1 0 1 1-2 0v-1a1 1 0 0 1 1-1zM4 12a1 1 0 0 1-1-1H2a1 1 0 1 1 0-2h1a1 1 0 1 1 1 1zm16 0a1 1 0 0 1-1-1h-1a1 1 0 1 1 0-2h1a1 1 0 1 1 1 2zM6.3 6.3a1 1 0 0 1 0 1.4l-.7.7a1 1 0 1 1-1.4-1.4l.7-.7a1 1 0 0 1 1.4 0zm12.4 12.4a1 1 0 0 1 0 1.4l-.7.7a1 1 0 1 1-1.4-1.4l.7-.7a1 1 0 0 1 1.4 0zM6.3 17.7a1 1 0 0 1-1.4 0l-.7-.7a1 1 0 1 1 1.4-1.4l.7.7a1 1 0 0 1 0 1.4zm10.8-9.8a1 1 0 0 1 0-1.4l.7-.7a1 1 0 1 1 1.4 1.4l-.7.7a1 1 0 0 1-1.4 0z"/>
          </svg>
          <svg class="Header-themeIcon Header-themeIcon--dark" width="18" height="18" viewBox="0 0 24 24" aria-hidden="true">
            <path fill="currentColor" d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z"/>
          </svg>
        </button>
        <button class="Header-burger js-drawer-open" type="button" aria-label="Open navigation" aria-expanded="false">
          <span></span><span></span><span></span>
        </button>
      </div>
    </div>
  </header>

  <div class="Drawer-scrim js-drawer-scrim" hidden></div>
  <aside class="Drawer js-drawer" aria-label="Site navigation" aria-hidden="true">
    <div class="Drawer-head">
      <a class="Header-logo" href="{{HOME}}">
        <img src="{{ASSETS}}/img/icon.png?v={{V}}" alt="" width="32" height="32">
        <span class="Header-wordmark">Jin</span>
      </a>
      <button class="Drawer-close js-drawer-close" type="button" aria-label="Close navigation">&times;</button>
    </div>
    <nav class="Drawer-nav">
      <ul class="Drawer-list">
        %s
      </ul>
    </nav>
  </aside>""" % (nav_markup(active, home), REPO, "".join(groups))


def footer_markup(home):
    columns = [
        ("Why Jin", [
            ("Design rules", "docs/roadmap.html#design-rules"),
            ("What is in", "docs/roadmap.html#what-is-in"),
            ("Architecture", "docs/architecture.html"),
            ("Roadmap", "docs/roadmap.html"),
        ]),
        ("Learn", [
            ("Getting started", "docs/tutorial.html"),
            ("Language tour", "docs/tour.html"),
            ("Type system", "docs/types.html"),
            ("Built-ins", "docs/builtins.html"),
        ]),
        ("Reference", [
            ("Records & methods", "docs/records.html"),
            ("Error handling", "docs/errors.html"),
            ("Projects & imports", "docs/modules.html"),
            ("Command line", "docs/cli.html"),
            ("FAQ", "docs/faq.html"),
        ]),
        ("Project", [
            ("GitHub", REPO),
            ("Source code", REPO + "/tree/main"),
            ("Examples", REPO + "/tree/main/examples"),
            ("Issues", REPO + "/issues"),
        ]),
    ]
    cols = []
    for title, links in columns:
        items = []
        for label, href in links:
            # An absolute link stays absolute; only a local one gets the
            # prefix that keeps the page one level down.
            target = href if href.startswith("http") else home + href
            items.append('<li><a href="%s">%s</a></li>' % (target, label))
        cols.append(
            '<div class="Footer-col"><h2 class="Footer-colTitle">%s</h2>'
            '<ul class="Footer-list">%s</ul></div>' % (title, "".join(items))
        )
    return """  <footer class="Footer">
    <div class="Container">
      <div class="Footer-cols">%s</div>
    </div>
    <div class="Container">
      <div class="Footer-bottom">
        <p class="Footer-copy">
          Jin is free and open source, released under the
          <a href="%s/blob/main/LICENSE">MIT licence</a> or the
          <a href="%s/blob/main/LICENSE-APACHE">Apache Licence 2.0</a>, at your
          option. Designed by <a href="https://github.com/jojin1709">JOJIN JOHN</a>
          and built in stages with AI coding agents. No parser generator, no
          third-party libraries.
        </p>
        <ul class="Footer-bottomList">
          <li><a href="%s">Source on GitHub</a></li>
          <li><a href="%s/issues">Report an issue</a></li>
          <li><a href="%s/blob/main/CONTRIBUTING.md">Contributing</a></li>
          <li><a href="%s/blob/main/CODE_OF_CONDUCT.md">Code of conduct</a></li>
          <li class="Footer-version">Version %s</li>
        </ul>
      </div>
    </div>
  </footer>""" % ("".join(cols), REPO, REPO, REPO, REPO, REPO, REPO, VERSION)


def sidebar_markup(current):
    groups = []
    for title, links in SIDEBAR:
        items = []
        for label, href in links:
            here = ' aria-current="page"' if href == current else ""
            items.append(
                '<li><a href="%s"%s>%s</a></li>' % (href, here, label)
            )
        groups.append(
            '<div class="Sidebar-group"><h2 class="Sidebar-groupTitle">%s</h2>'
            '<ul class="Sidebar-list">%s</ul></div>' % (title, "".join(items))
        )
    return "".join(groups)


# ------------------------------------------------------------------ the build


def read(path):
    with open(path, encoding="utf-8") as handle:
        return handle.read()


def wrap_tables(body):
    """A wide table needs somewhere to scroll on a narrow screen, or it
    pushes the page sideways. Every table goes into a box that scrolls."""
    return re.sub(
        r"<table class=\"Table\">(.*?)</table>",
        r'<div class="TableWrap"><table class="Table">\1</table></div>',
        body,
        flags=re.S,
    )


def build(name, body, out_path, title, description, active="",
          assets="assets", home="", extra_head="", extra_script="",
          extra_class="", sidebar="", page_path=""):
    if sidebar:
        # The docs pages bring their own <div class="DocLayout"> wrapper; the
        # sidebar is dropped in beside the content, not written into the page.
        body = re.sub(
            r'(<div class="DocLayout[^"]*">)',
            lambda m: m.group(1) + '\n  <aside class="Sidebar">' + sidebar +
            "</aside>",
            body,
            count=1,
        )
    template = read(os.path.join(TPL, "base.html"))
    body = wrap_tables(body)
    # The header and the footer are inserted before the placeholders are
    # filled in, because they contain links of their own.
    template = sub(template, "NAVBAR", header_markup(active, home))
    template = sub(template, "FOOTER", footer_markup(home))
    template = sub(template, "TITLE", title)
    template = sub(template, "DESCRIPTION", description)
    template = sub(template, "BODY", body)
    template = sub(template, "ASSETS", assets)
    template = sub(template, "HOME", home)
    template = sub(template, "SIDEBAR", sidebar)
    template = sub(template, "SITE_URL", SITE_URL)
    template = sub(template, "PAGE_PATH", page_path)
    template = sub(template, "V", BUILD)
    template = sub(template, "EXTRA_HEAD", extra_head)
    template = sub(template, "EXTRA_SCRIPT", extra_script)
    template = sub(template, "EXTRA_CLASS", extra_class)
    template = sub(template, "VERSION", VERSION)
    os.makedirs(os.path.dirname(out_path), exist_ok=True)
    with open(out_path, "w", encoding="utf-8", newline="\n") as handle:
        handle.write(template)
    print("wrote %s" % os.path.relpath(out_path, ROOT))

DOC_PAGES = [
    ("docs-index", "index", "Documentation", "index.html", ""),
    ("tutorial", "tutorial", "Getting Started", "tutorial.html", "tutorial.html"),
    ("tour", "tour", "Language Tour", "tour.html", "tour.html"),
    ("types", "types", "Type System", "types.html", "types.html"),
    ("records", "records", "Records and Methods", "records.html", "records.html"),
    ("errors", "errors", "Error Handling", "errors.html", "errors.html"),
    ("builtins", "builtins", "Built-ins", "builtins.html", "builtins.html"),
    ("modules", "modules", "Projects and Imports", "modules.html", "modules.html"),
    ("cli", "cli", "Command Line", "cli.html", "cli.html"),
    ("architecture", "architecture", "Architecture", "architecture.html", "architecture.html"),
    ("roadmap", "roadmap", "Status and Roadmap", "roadmap.html", "roadmap.html"),
    ("faq", "faq", "Frequently Asked Questions", "faq.html", "faq.html"),
]

DESCRIPTIONS = {
    "index": "The index of everything written down about the Jin programming language.",
    "tutorial": "Install Jin, write your first program, and understand the ten-minute version of the language.",
    "tour": "Every construct in Jin: values, control flow, functions, closures, records, maps and error handling.",
    "types": "Jin's optional type annotations, and every check the type checker runs before your program starts.",
    "records": "User-defined record types in Jin: fields, constructors, receiver methods and bound methods.",
    "errors": "Catching runtime errors in Jin with try and catch, raising your own with error(), and re-raising.",
    "builtins": "The Jin standard library: every built-in function, what it takes and what it returns.",
    "modules": "Splitting a Jin program across files, importing them under an alias, and building a project.",
    "cli": "Every jin subcommand: run, interpret, check, build, new, compile, parse and tokens.",
    "architecture": "How a Jin program becomes running bytecode: the lexer, the parser, the type checker, the compiler and the VM.",
    "roadmap": "What Jin can do today, what it cannot do yet, and what comes next.",
    "faq": "The questions that come up about Jin, and the short answers.",
}

# ------------------------------------------------------------------ markdown


def escape_html(text):
    return (
        text.replace("&", "&amp;")
        .replace("<", "&lt;")
        .replace(">", "&gt;")
        .replace('"', "&quot;")
    )


def inline_markdown(text):
    """The inline markup the FAQ uses: code, bold and links."""
    out = []
    for part in re.split(r"(`[^`]+`)", text):
        if len(part) > 1 and part.startswith("`") and part.endswith("`"):
            out.append("<code>%s</code>" % escape_html(part[1:-1]))
            continue
        part = escape_html(part)
        part = re.sub(r"\*\*([^*]+)\*\*", r"<strong>\1</strong>", part)
        part = re.sub(r"\[([^\]]+)\]\(([^)\s]+)\)", r'<a href="\2">\1</a>', part)
        out.append(part)
    return "".join(out)


def render_markdown(text):
    """Renders the small set of markdown the FAQ uses: headings, paragraphs
    and bullet lists. Anything it does not know about stops the build rather
    than being quietly dropped, so the page cannot drift from its source."""
    out = []
    lines = text.split("\n")
    i = 0

    while i < len(lines):
        line = lines[i].rstrip()

        if not line.strip():
            i += 1
            continue

        if line.startswith("### "):
            out.append("<h3>%s</h3>" % inline_markdown(line[4:].strip()))
            i += 1
            continue

        if line.startswith("## "):
            out.append("<h2>%s</h2>" % inline_markdown(line[3:].strip()))
            i += 1
            continue

        if line.startswith("# "):
            # The title is the page title, not a heading in the page.
            i += 1
            continue

        if line.startswith("- "):
            items = []
            while i < len(lines) and lines[i].startswith("- "):
                items.append("<li>%s</li>" % inline_markdown(lines[i][2:].strip()))
                i += 1
            out.append('<ul class="Bullets">%s</ul>' % "".join(items))
            continue

        if re.match(r"^\d+\. ", line):
            items = []
            while i < len(lines) and re.match(r"^\d+\. ", lines[i]):
                items.append("<li>%s</li>" % inline_markdown(re.sub(r"^\d+\. ", "", lines[i]).strip()))
                i += 1
            out.append('<ol class="Steps">%s</ol>' % "".join(items))
            continue

        if line.startswith((">", "|", "```", "    ", "\t", "---", "***")):
            sys.exit("FAQ.md uses markdown the builder does not know: %r" % line)

        # Anything else is a paragraph, which runs to the next blank line.
        para = []
        while i < len(lines) and lines[i].strip() and not re.match(
            r"^(#|-\s|\d+\.\s|>|```|\|)", lines[i]
        ):
            para.append(lines[i].strip())
            i += 1
        if not para:
            sys.exit("FAQ.md uses markdown the builder does not know: %r" % line)
        out.append("<p>%s</p>" % inline_markdown(" ".join(para)))

    return "\n".join(out)


def faq_body():
    """The FAQ page, rendered from FAQ.md so that there is one source."""
    source = os.path.join(ROOT, "FAQ.md")
    if not os.path.exists(source):
        sys.exit("missing %s" % source)
    with open(source, encoding="utf-8") as handle:
        text = handle.read()
    return (
        '<div class="DocHeader">\n'
        '  <p class="DocHeader-crumb"><a href="index.html">Docs</a> / FAQ</p>\n'
        '  <h1>Frequently Asked Questions</h1>\n'
        "</div>\n\n"
        '<div class="DocLayout">\n'
        '  <div class="DocMain">\n\n'
        + render_markdown(text) +
        "\n  </div>\n</div>\n"
    )



def main():
    if not os.path.isdir(PAGES):
        sys.exit("missing %s" % PAGES)

    # landing page
    body = read(os.path.join(PAGES, "index.body.html"))
    build(
        "index", body,
        os.path.join(SITE, "index.html"),
        "The Jin Programming Language",
        "Jin is a small programming language that is easy to write, checked "
        "before it runs, and as fast as CPython on call-heavy code.",
        active="", assets="assets", home="", page_path="index.html",
    )

    # documentation hub
    body = read(os.path.join(PAGES, "docs-index.body.html"))
    build(
        "docs-index", body,
        os.path.join(OUT_DOCS, "index.html"),
        "Documentation — The Jin Programming Language",
        DESCRIPTIONS["index"], active="docs",
        assets="../assets", home="../", page_path="docs/index.html",
    )

    # the rest of the documentation
    for stem, slug, title, filename, current in DOC_PAGES:
        if stem == "docs-index":
            continue
        if stem == "faq":
            # The FAQ page is rendered from FAQ.md, so that the file at the
            # root of the repository and the page on the website are the
            # same text.
            body = faq_body()
        else:
            path = os.path.join(PAGES, stem + ".body.html")
            if not os.path.exists(path):
                print("skipped %s (not written yet)" % filename)
                continue
            body = read(path)
        build(
            stem, body,
            os.path.join(OUT_DOCS, filename),
            title + " — The Jin Programming Language",
            DESCRIPTIONS[slug], active="docs",
            assets="../assets", home="../",
            sidebar=sidebar_markup(current),
            page_path="docs/" + filename,
        )


if __name__ == "__main__":
    main()
