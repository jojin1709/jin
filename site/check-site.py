"""Checks the built site: every local link and asset must exist, every page
must have a title and a description, and the code blocks must be balanced.

    python site/tools/check-site.py
"""

import os
import re
import sys

ROOT = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "docs")

LINK = re.compile(r'(?:href|src)="([^"]+)"')
TITLE = re.compile(r"<title>(.*?)</title>", re.S)
DESC = re.compile(r'<meta name="description" content="(.*?)">', re.S)

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

for problem in problems:
    print("  " + problem)

print("")
print("  %d pages checked, %d problems" % (len(pages), len(problems)))
sys.exit(1 if problems else 0)
