/* ==========================================================================
   Jin — jin.dev
   Theme, navigation, code blocks, the in-page search and the playground.
   No dependencies, no build step, no network calls.
   ========================================================================== */

(function () {
  "use strict";

  /* ------------------------------------------------------------- theme */

  var THEME_KEY = "jin-theme";
  var ORDER = ["auto", "light", "dark"];

  function storedTheme() {
    try {
      return localStorage.getItem(THEME_KEY) || "auto";
    } catch (e) {
      return "auto";
    }
  }

  function applyTheme(theme) {
    var dark =
      theme === "dark" ||
      (theme === "auto" &&
        window.matchMedia("(prefers-color-scheme: dark)").matches);
    document.documentElement.setAttribute("data-theme", dark ? "dark" : "light");
  }

  function setTheme(theme) {
    applyTheme(theme);
    try {
      localStorage.setItem(THEME_KEY, theme);
    } catch (e) { /* private mode */ }
    var button = document.querySelector(".js-theme-toggle");
    if (button) {
      button.setAttribute(
        "aria-label",
        "Colour theme: " + theme + ". Click to change."
      );
      button.setAttribute("title", "Colour theme: " + theme);
    }
  }

  function initTheme() {
    setTheme(storedTheme());
    var button = document.querySelector(".js-theme-toggle");
    if (!button) return;
    button.addEventListener("click", function () {
      var current = storedTheme();
      var next = ORDER[(ORDER.indexOf(current) + 1) % ORDER.length];
      setTheme(next);
    });
    window
      .matchMedia("(prefers-color-scheme: dark)")
      .addEventListener("change", function () {
        if (storedTheme() === "auto") applyTheme("auto");
      });
  }

  /* ------------------------------------------------------------- drawer */

  function initDrawer() {
    var drawer = document.querySelector(".js-drawer");
    var scrim = document.querySelector(".js-drawer-scrim");
    var opener = document.querySelector(".js-drawer-open");
    var closer = document.querySelector(".js-drawer-close");
    if (!drawer || !opener) return;

    var lastFocus = null;

    function focusables() {
      return drawer.querySelectorAll(
        'a[href], button:not([disabled]), input, select, textarea, summary, [tabindex]:not([tabindex="-1"])'
      );
    }

    function open() {
      lastFocus = document.activeElement;
      drawer.classList.add("is-open");
      drawer.setAttribute("aria-hidden", "false");
      opener.setAttribute("aria-expanded", "true");
      if (scrim) scrim.hidden = false;
      document.body.style.overflow = "hidden";
      var first = focusables()[0];
      if (first) first.focus();
    }

    function close() {
      drawer.classList.remove("is-open");
      drawer.setAttribute("aria-hidden", "true");
      opener.setAttribute("aria-expanded", "false");
      if (scrim) scrim.hidden = true;
      document.body.style.overflow = "";
      if (lastFocus) lastFocus.focus();
    }

    opener.addEventListener("click", open);
    if (closer) closer.addEventListener("click", close);
    if (scrim) scrim.addEventListener("click", close);

    document.addEventListener("keydown", function (event) {
      if (!drawer.classList.contains("is-open")) return;
      if (event.key === "Escape") {
        close();
        return;
      }
      if (event.key !== "Tab") return;
      var items = focusables();
      if (!items.length) return;
      var first = items[0];
      var last = items[items.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    });
  }

  /* ---------------------------------------------------------- submenus */

  function initSubmenus() {
    var triggers = document.querySelectorAll(".Header-submenuTrigger");
    Array.prototype.forEach.call(triggers, function (trigger) {
      trigger.addEventListener("click", function (event) {
        event.stopPropagation();
        var open = trigger.getAttribute("aria-expanded") === "true";
        Array.prototype.forEach.call(triggers, function (other) {
          var menu = document.getElementById(
            other.getAttribute("aria-controls")
          );
          other.setAttribute("aria-expanded", "false");
          if (menu) menu.hidden = true;
        });
        if (!open) {
          trigger.setAttribute("aria-expanded", "true");
          var menu = document.getElementById(
            trigger.getAttribute("aria-controls")
          );
          if (menu) menu.hidden = false;
        }
      });
    });

    document.addEventListener("click", function () {
      Array.prototype.forEach.call(triggers, function (trigger) {
        var menu = document.getElementById(
          trigger.getAttribute("aria-controls")
        );
        trigger.setAttribute("aria-expanded", "false");
        if (menu) menu.hidden = true;
      });
    });

    document.addEventListener("keydown", function (event) {
      if (event.key === "Escape") {
        Array.prototype.forEach.call(triggers, function (trigger) {
          var menu = document.getElementById(
            trigger.getAttribute("aria-controls")
          );
          trigger.setAttribute("aria-expanded", "false");
          if (menu) menu.hidden = true;
        });
      }
    });
  }

  /* ------------------------------------------------------ code blocks */

  var JIN_KEYWORDS = [
    "import", "as", "struct", "fn", "let", "if", "else", "for", "in",
    "while", "return", "break", "continue", "true", "false", "nil",
    "try", "catch"
  ];

  var JIN_TYPES = ["int", "float", "str", "bool", "array", "map", "any"];

  function escapeHtml(text) {
    return text
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;");
  }

  /* A small Jin highlighter: keywords, types, strings (with {interpolation}),
     numbers, comments and calls. It is deliberately simple — it must never
     change what the code means, only colour it. */
  function highlightJin(source) {
    var out = "";
    var i = 0;
    var length = source.length;

    function push(cls, text) {
      out += '<span class="' + cls + '">' + escapeHtml(text) + "</span>";
    }

    while (i < length) {
      var ch = source[i];

      if (ch === "#") {
        var end = source.indexOf("\n", i);
        if (end === -1) end = length;
        push("tk-com", source.slice(i, end));
        i = end;
        continue;
      }

      if (ch === '"') {
        var buf = '"';
        i++;
        while (i < length) {
          if (source[i] === "\\" && i + 1 < length) {
            buf += source[i] + source[i + 1];
            i += 2;
            continue;
          }
          if (source[i] === '"') {
            buf += '"';
            i++;
            break;
          }
          if (source[i] === "{" && source[i + 1] === "{") {
            buf += "{{";
            i += 2;
            continue;
          }
          if (source[i] === "{") {
            var depth = 1;
            var inner = "{";
            i++;
            while (i < length && depth > 0) {
              if (source[i] === "{") depth++;
              if (source[i] === "}") {
                depth--;
                if (depth === 0) {
                  inner += "}";
                  i++;
                  break;
                }
              }
              inner += source[i];
              i++;
            }
            buf += '<span class="tk-interp">' + escapeHtml(inner) + "</span>";
            continue;
          }
          buf += source[i];
          i++;
        }
        push("tk-str", buf);
        continue;
      }

      if (/[0-9]/.test(ch)) {
        var num = /^[0-9]+(\.[0-9]+)?/.exec(source.slice(i))[0];
        push("tk-num", num);
        i += num.length;
        continue;
      }

      if (/[A-Za-z_]/.test(ch)) {
        var word = /^[A-Za-z_][A-Za-z0-9_]*/.exec(source.slice(i))[0];
        var after = source.slice(i + word.length);
        var before = source.slice(0, i);
        if (JIN_KEYWORDS.indexOf(word) !== -1) {
          push("tk-kw", word);
        } else if (
          JIN_TYPES.indexOf(word) !== -1 &&
          (/^\s*:/.test(after) || /^\s*->/.test(after))
        ) {
          push("tk-type", word);
        } else if (/^\s*\(/.test(after)) {
          push("tk-fn", word);
        } else if (
          /[A-Za-z0-9_]/.test(before.charAt(before.length - 1) || "")
        ) {
          out += escapeHtml(word);
        } else {
          push("tk-type", word);
        }
        i += word.length;
        continue;
      }

      if ("+-*/%<>=!&|".indexOf(ch) !== -1) {
        var op = /^(==|!=|<=|>=|&&|\|\|)/.exec(source.slice(i));
        if (op) {
          push("tk-op", op[0]);
          i += op[0].length;
          continue;
        }
        push("tk-op", ch);
        i++;
        continue;
      }

      out += escapeHtml(ch);
      i++;
    }

    return out;
  }

  function highlightShell(source) {
    return source
      .split("\n")
      .map(function (line) {
        if (/^\s*#/.test(line)) {
          return '<span class="tk-com">' + escapeHtml(line) + "</span>";
        }
        if (/^\s*\$/.test(line)) {
          return (
            '<span class="tk-op">$</span>' +
            escapeHtml(line.replace(/^\s*\$\s*/, ""))
          );
        }
        return escapeHtml(line);
      })
      .join("\n");
  }

  function initCodeBlocks() {
    var blocks = document.querySelectorAll(".Code");

    Array.prototype.forEach.call(blocks, function (block) {
      var code = block.querySelector("code");
      if (!code) return;

      var lang = block.getAttribute("data-lang");
      if (lang === "jin") {
        code.innerHTML = highlightJin(code.textContent);
      } else if (lang === "shell") {
        code.innerHTML = highlightShell(code.textContent);
      }

      if (lang && lang !== "output" && lang !== "text") {
        var label = document.createElement("span");
        label.className = "Code-label";
        label.textContent = lang;
        block.appendChild(label);
      }

      var button = document.createElement("button");
      button.className = "Code-copy";
      button.type = "button";
      button.textContent = "Copy";
      button.setAttribute("aria-label", "Copy this code to the clipboard");
      button.addEventListener("click", function () {
        var text = code.textContent;
        var done = function () {
          button.textContent = "Copied";
          button.classList.add("is-copied");
          window.setTimeout(function () {
            button.textContent = "Copy";
            button.classList.remove("is-copied");
          }, 1600);
        };
        if (navigator.clipboard && navigator.clipboard.writeText) {
          navigator.clipboard.writeText(text).then(done, function () {});
        } else {
          var area = document.createElement("textarea");
          area.value = text;
          document.body.appendChild(area);
          area.select();
          try {
            document.execCommand("copy");
            done();
          } catch (e) { /* clipboard unavailable */ }
          document.body.removeChild(area);
        }
      });
      block.appendChild(button);
    });
  }

  /* ---------------------------------------------------- sticky header */

  function initStickyHeader() {
    var header = document.querySelector(".js-header");
    if (!header) return;
    var onScroll = function () {
      header.classList.toggle("is-scrolled", window.scrollY > 4);
    };
    onScroll();
    window.addEventListener("scroll", onScroll, { passive: true });
  }

  /* ------------------------------------------------------- playground */

  function initPlayground() {
    var editor = document.querySelector(".js-playground-editor");
    var output = document.querySelector(".js-playground-output");
    var run = document.querySelector(".js-playground-run");
    var reset = document.querySelector(".js-playground-reset");
    var picker = document.querySelector(".js-playground-examples");
    if (!editor || !output) return;

    var examples = window.JIN_EXAMPLES || {};
    var current = "";

    function show(text, isError) {
      output.textContent = text;
      output.classList.toggle("is-error", !!isError);
    }

    function runCode() {
      if (!window.Jin) {
        show("The playground engine did not load.", true);
        return;
      }
      var result;
      try {
        result = window.Jin.run(editor.value);
      } catch (error) {
        show(String(error && error.message ? error.message : error), true);
        return;
      }
      if (result && result.error) {
        show(result.error, true);
        return;
      }
      show(result && result.output ? result.output : "", false);
    }

    if (picker) {
      picker.addEventListener("change", function () {
        var code = examples[picker.value];
        if (code === undefined) return;
        editor.value = code;
        current = picker.value;
        runCode();
      });
    }

    if (run) {
      run.addEventListener("click", function () {
        run.disabled = true;
        run.textContent = "Running…";
        window.setTimeout(function () {
          runCode();
          run.disabled = false;
          run.textContent = "Run";
        }, 10);
      });
    }

    if (reset) {
      reset.addEventListener("click", function () {
        if (picker && examples[picker.value] !== undefined) {
          editor.value = examples[picker.value];
        }
        show("", false);
      });
    }

    editor.addEventListener("keydown", function (event) {
      if (event.key === "Enter" && (event.ctrlKey || event.metaKey)) {
        event.preventDefault();
        runCode();
      }
    });

    if (picker && examples[picker.value] !== undefined) {
      editor.value = examples[picker.value];
    }
    runCode();
  }

  /* ------------------------------------------------------------- boot */

  function boot() {
    initTheme();
    initDrawer();
    initSubmenus();
    initCodeBlocks();
    initStickyHeader();
    initPlayground();
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", boot);
  } else {
    boot();
  }
})();
