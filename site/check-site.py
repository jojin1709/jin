"""Checks the built site: every local link and asset must exist, every page
must have a title and a description, and the code blocks must be balanced.

It also checks that the FAQ page on the site carries every question from
FAQ.md, so that the file at the root of the repository and the page cannot
drift apart.

    python site/check-site.py
"""

import os
import re
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.join(os.path.dirname(HERE), "docs")
FAQ_SOURCE = os.path.join(os.path.dirname(HERE), "FAQ.md")
FAQ_PAGE = os.path.join(ROOT, "docs", "faq.html")

LINK = re.compile(r'(?:href|src)="([^"]+)"')
TITLE = re.compile(r"<title>(.*?)</title>", re.S)
DESC = re.compile(r'<meta name="description" content="(.*?)">', re.S)
HEADING = re.compile(r"^##\s+(.*?)\s*$", re.M)

problems = []
pages = []

for base, dirs, files in os.walk(ROOT):
    if os.path.basename(base) == "tools":
        dirs[:] = []
        continue
    for name in files:
        if name.endswith(".html"):
            pages.append(os.path.join(base, name))

for page in sorted(pages):
    rel = os.path.relpath(page, ROOT)
    with open(page, encoding="utf-8") as handle:
        html = handle.read()

    if not TITLE.search(html):
        problems.append("%s: no <title>" % rel)
    if not DESC.search(html):
        problems.append("%s: no meta description" % rel)
    if 'id="main-content"' not in html:
        problems.append("%s: no main landmark" % rel)
    if html.count("<pre") != html.count("</pre>"):
        problems.append("%s: unbalanced <pre>" % rel)
    if html.count("<section") != html.count("</section>"):
        problems.append("%s: unbalanced <section>" % rel)

    for target in LINK.findall(html):
        if target.startswith(("http://", "https://", "mailto:", "#", "data:")):
            continue
        path = target.split("#")[0].split("?")[0]
        if not path:
            continue
        resolved = os.path.normpath(os.path.join(os.path.dirname(page), path))
        if not os.path.exists(resolved):
            problems.append("%s: missing %s" % (rel, target))

# The FAQ page is rendered from FAQ.md, so every question in the file has to be
# on the page. The answers are checked by the example checker; this is about
# the questions themselves.
if os.path.exists(FAQ_SOURCE) and os.path.exists(FAQ_PAGE):
    with open(FAQ_SOURCE, encoding="utf-8") as handle:
        source = handle.read()
    with open(FAQ_PAGE, encoding="utf-8") as handle:
        page = handle.read()

    questions = HEADING.findall(source)
    if not questions:
        problems.append("FAQ.md: no questions found")
    for question in questions:
        if question not in page:
            problems.append("docs/docs/faq.html: the question %r is not on the page" % question)
    if "<h1>Frequently Asked Questions</h1>" not in page:
        problems.append("docs/docs/faq.html: the page has no heading")
else:
    problems.append("FAQ.md or docs/docs/faq.html is missing")

for problem in problems:
    print("  " + problem)

print("")
print("  %d pages checked, %d problems" % (len(pages), len(problems)))
sys.exit(1 if problems else 0)
