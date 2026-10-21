/* Runs every browser-playground example through the interpreter and prints
   the result, so the JavaScript engine can be checked without a browser.
       node site/tools/test-interpreter.js
   The expected output is in site/tools/expected.txt. */

const fs = require("fs");
const path = require("path");
const vm = require("vm");

const root = path.join(__dirname, "..");
const sandbox = { console };
sandbox.window = sandbox;
sandbox.globalThis = sandbox;
vm.createContext(sandbox);

for (const file of ["assets/js/jin.js", "assets/js/examples.js"]) {
  vm.runInContext(fs.readFileSync(path.join(root, file), "utf8"), sandbox, {
    filename: file
  });
}

const examples = sandbox.window.JIN_EXAMPLES;
let failed = 0;

for (const [name, code] of Object.entries(examples)) {
  const result = sandbox.window.Jin.run(code);
  console.log("=== " + name + " ===");
  if (result.error) {
    failed++;
    console.log("ERROR: " + result.error);
  } else {
    console.log(result.output);
  }
  console.log("");
}

process.exit(failed ? 1 : 0);
