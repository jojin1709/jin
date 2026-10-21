/* Walks every page of the site, takes a picture of each one, and reports
   anything that is broken: a missing asset, a console error, an image in the
   wrong shape, or text that runs off the side of the page.

     node site/screenshot.js            (all pages, at 1440 and 390 wide)
*/

const fs = require("fs");
const path = require("path");
const { chromium } = require("playwright");

const BASE = process.argv[2] || "http://localhost:8767";
const OUT = process.argv[3] || "shots";
const WIDTHS = (process.argv[4] || "1440,390").split(",").map(Number);

const PAGES = [
  ["index.html", "home"],
  ["docs/index.html", "docs-hub"],
  ["docs/tutorial.html", "tutorial"],
  ["docs/tour.html", "tour"],
  ["docs/types.html", "types"],
  ["docs/records.html", "records"],
  ["docs/errors.html", "errors"],
  ["docs/builtins.html", "builtins"],
  ["docs/modules.html", "modules"],
  ["docs/cli.html", "cli"],
  ["docs/architecture.html", "architecture"],
  ["docs/roadmap.html", "roadmap"],
];

(async () => {
  fs.mkdirSync(OUT, { recursive: true });
  const browser = await chromium.launch();
  let faults = 0;

  for (const width of WIDTHS) {
    const context = await browser.newContext({
      viewport: { width, height: 900 },
      colorScheme: width > 700 ? "light" : "dark",
    });

    for (const [file, name] of PAGES) {
      const page = await context.newPage();
      const problems = [];

      page.on("console", (m) => {
        if (m.type() === "error") problems.push("console: " + m.text());
      });
      page.on("pageerror", (e) => problems.push("pageerror: " + e.message));
      page.on("requestfailed", (r) =>
        problems.push("failed: " + r.url() + " " + r.failure().errorText)
      );

      await page.goto(BASE + "/" + file, { waitUntil: "networkidle" });
      await page.screenshot({
        path: path.join(OUT, name + "-" + width + ".png"),
        fullPage: false,
      });

      const found = await page.evaluate(() => {
        const out = [];
        document.querySelectorAll("img").forEach((img) => {
          if (img.closest(".Drawer")) return;
          const rect = img.getBoundingClientRect();
          if (rect.width === 0 || rect.height === 0) {
            out.push("zero size: " + img.getAttribute("src"));
            return;
          }
          const natural = img.naturalWidth / img.naturalHeight;
          const shown = rect.width / rect.height;
          if (natural > 0 && Math.abs(natural - shown) > 0.06) {
            out.push("wrong shape: " + img.getAttribute("src"));
          }
          if (rect.right > window.innerWidth + 1) {
            out.push("overflows the page: " + img.getAttribute("src"));
          }
        });
        if (document.documentElement.scrollWidth > window.innerWidth + 1) {
          out.push("the page scrolls sideways");
        }
        return out;
      });

      if (problems.length || found.length) {
        faults++;
        console.log(file + " @ " + width);
        problems.concat(found).forEach((p) => console.log("  " + p));
      }
      await page.close();
    }
    await context.close();
  }

  await browser.close();
  console.log(faults ? faults + " pages with problems" : "no problems found");
  console.log("pictures in " + OUT + "/");
  process.exit(faults ? 1 : 0);
})();
