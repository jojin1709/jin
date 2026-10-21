/* ==========================================================================
   Jin in the browser — a tree-walking interpreter for a subset of Jin.
   Used only by the landing-page playground. It is not the real language:
   it covers the constructs a first-time visitor meets, and says so plainly
   when it meets something it does not implement.
   ========================================================================== */

(function (global) {
  "use strict";

  /* ------------------------------------------------------------- values */

  function Int(v) { return { t: "int", v: v }; }
  function Float(v) { return { t: "float", v: v }; }
  function Str(v) { return { t: "str", v: v }; }
  function Bool(v) { return { t: "bool", v: v }; }
  function Nil() { return { t: "nil", v: null }; }
  function Arr(v) { return { t: "arr", v: v }; }
  function Rec(type, fields, order) {
    return { t: "rec", type: type, f: fields, order: order || Object.keys(fields) };
  }

  function Map(pairs) {
    this.t = "map";
    this.order = [];
    this.data = {};
    if (pairs) {
      for (var i = 0; i < pairs.length; i++) {
        this.set(pairs[i][0], pairs[i][1]);
      }
    }
  }

  Map.prototype.key = function (value) {
    return value.t + ":" + String(value.v);
  };

  Map.prototype.set = function (key, value) {
    var k = this.key(key);
    if (!(k in this.data)) this.order.push(key);
    this.data[k] = value;
  };

  Map.prototype.get = function (key) {
    var k = this.key(key);
    return k in this.data ? this.data[k] : null;
  };

  Map.prototype.has = function (key) {
    return this.key(key) in this.data;
  };

  Map.prototype.del = function (key) {
    var k = this.key(key);
    if (!(k in this.data)) return false;
    delete this.data[k];
    for (var i = 0; i < this.order.length; i++) {
      if (this.key(this.order[i]) === k) {
        this.order.splice(i, 1);
        break;
      }
    }
    return true;
  };

  function FnObject(node, env, name) {
    this.t = "fn";
    this.node = node;
    this.env = env;
    this.name = name || "fn";
  }

  function Builtin(name, fn, arity) {
    this.t = "builtin";
    this.name = name;
    this.fn = fn;
    this.arity = arity;
  }

  function Method(recv, fn) {
    this.t = "method";
    this.recv = recv;
    this.fn = fn;
  }

  var NIL = Nil();
  var TRUE = Bool(true);
  var FALSE = Bool(false);

  function isInt(o) { return o.t === "int"; }
  function isFloat(o) { return o.t === "float"; }
  function isNum(o) { return o.t === "int" || o.t === "float"; }
  function isStr(o) { return o.t === "str"; }
  function isArr(o) { return o.t === "arr"; }
  function isMap(o) { return o.t === "map"; }
  function isRec(o) { return o.t === "rec"; }
  function isBool(o) { return o.t === "bool"; }
  function isNil(o) { return o.t === "nil"; }
  function isCallable(o) {
    return o.t === "fn" || o.t === "builtin" || o.t === "method";
  }

  function typeName(o) {
    switch (o.t) {
      case "int": return "int";
      case "float": return "float";
      case "str": return "str";
      case "bool": return "bool";
      case "nil": return "nil";
      case "arr": return "array";
      case "map": return "map";
      case "rec": return o.type;
      case "fn": case "builtin": case "method": return "function";
      default: return o.t;
    }
  }

  function formatFloat(n) {
    if (n === Infinity) return "Inf";
    if (n === -Infinity) return "-Inf";
    if (isNaN(n)) return "NaN";
    return String(n);
  }

  /* repr renders a value the way it appears inside a collection, so a string
     keeps its quotes. inspect renders it the way it appears on its own. */
  function repr(o) {
    switch (o.t) {
      case "int": return String(o.v);
      case "float": return formatFloat(o.v);
      case "str": return '"' + o.v.replace(/\\/g, "\\\\").replace(/"/g, '\\"') + '"';
      case "bool": return o.v ? "true" : "false";
      case "nil": return "nil";
      case "arr":
        return "[" + o.v.map(repr).join(", ") + "]";
      case "map": {
        var parts = [];
        for (var i = 0; i < o.order.length; i++) {
          parts.push(repr(o.order[i]) + ": " + repr(o.data[o.key(o.order[i])]));
        }
        return "{" + parts.join(", ") + "}";
      }
      case "rec": {
        var fields = [];
        var names = o.order || Object.keys(o.f);
        for (var j = 0; j < names.length; j++) {
          fields.push(names[j] + ": " + repr(o.f[names[j]]));
        }
        return o.type + "{" + fields.join(", ") + "}";
      }
      case "fn": return "<fn " + (o.name || "") + ">";
      case "builtin": return "<builtin " + o.name + ">";
      case "method": return "<method>";
      default:
        return "function";
    }
  }

  function inspect(o) {
    if (isStr(o)) return o.v;
    return repr(o);
  }

  /* ------------------------------------------------------------- errors */

  function JinError(message) {
    this.name = "JinError";
    this.message = message;
  }
  JinError.prototype = Object.create(Error.prototype);

  /* -------------------------------------------------------------- lexer */

  var KEYWORDS = {
    import: 1, as: 1, struct: 1, fn: 1, let: 1, if: 1, else: 1, for: 1,
    in: 1, while: 1, return: 1, break: 1, continue: 1, true: 1, false: 1,
    nil: 1, try: 1, catch: 1
  };

  var PUNCT = [
    "==", "!=", "<=", ">=", "&&", "||", "->",
    "+", "-", "*", "/", "%", "!", "<", ">", "=", "&", "|",
    "(", ")", "{", "}", "[", "]", ",", ";", ":", "."
  ];

  /* Reads a string literal, splitting out any {interpolations}. */
  function scanString(source, start, line) {
    var triple = source.slice(start, start + 3) === '"""';
    var quote = triple ? '"""' : '"';
    var i = start + quote.length;
    var parts = [];
    var text = "";

    function flush() {
      if (text !== "") {
        parts.push({ kind: "text", value: text });
        text = "";
      }
    }

    while (i < source.length) {
      if (!triple && source[i] === "\\" && i + 1 < source.length) {
        var esc = source[i + 1];
        if (esc === "n") text += "\n";
        else if (esc === "t") text += "\t";
        else if (esc === "r") text += "\r";
        else if (esc === "\\") text += "\\";
        else if (esc === '"') text += '"';
        else if (esc === "{") text += "{";
        else text += esc;
        i += 2;
        continue;
      }

      if (source.slice(i, i + quote.length) === quote) {
        i += quote.length;
        break;
      }

      if (!triple && source[i] === "\n") {
        throw new JinError("unterminated string on line " + line);
      }

      if (source[i] === "{" && source[i + 1] === "{") {
        text += "{";
        i += 2;
        continue;
      }

      if (source[i] === "}" && source[i + 1] === "}") {
        text += "}";
        i += 2;
        continue;
      }

      if (source[i] === "{") {
        var depth = 1;
        var j = i + 1;
        var expr = "";
        while (j < source.length && depth > 0) {
          var c = source[j];
          if (c === '"') {
            expr += c;
            j++;
            while (j < source.length && source[j] !== '"') {
              if (source[j] === "\\") {
                expr += source[j] + source[j + 1];
                j += 2;
                continue;
              }
              expr += source[j];
              j++;
            }
            expr += '"';
            j++;
            continue;
          }
          if (c === "{") depth++;
          if (c === "}") {
            depth--;
            if (depth === 0) break;
          }
          expr += c;
          j++;
        }
        if (depth !== 0) {
          throw new JinError("unterminated { in string on line " + line);
        }
        flush();
        parts.push({ kind: "expr", source: expr });
        i = j + 1;
        continue;
      }

      if (source[i] === "\n") line++;
      text += source[i];
      i++;
    }

    flush();

    if (triple && parts.length && parts[0].kind === "text" &&
        parts[0].value.charAt(0) === "\n") {
      parts[0].value = parts[0].value.slice(1);
    }

    return { parts: parts, end: i, line: line };
  }

  function lex(source) {
    var tokens = [];
    var i = 0;
    var line = 1;
    var col = 1;
    var length = source.length;

    function push(type, value, startCol, width) {
      tokens.push({ type: type, value: value, line: line, col: startCol });
      col += width;
    }

    while (i < length) {
      var ch = source[i];

      if (ch === "\n") { line++; col = 1; i++; continue; }
      if (ch === " " || ch === "\t" || ch === "\r") { i++; col++; continue; }

      if (ch === "#") {
        while (i < length && source[i] !== "\n") i++;
        continue;
      }

      if (ch === '"') {
        var startCol = col;
        var scanned = scanString(source, i, line);
        var startLine = line;
        line = scanned.line;
        tokens.push({
          type: "string",
          value: "",
          parts: scanned.parts,
          line: startLine,
          col: startCol
        });
        col = startCol + (scanned.end - i);
        i = scanned.end;
        continue;
      }

      if (/[0-9]/.test(ch)) {
        var num = /^[0-9]+(\.[0-9]+)?/.exec(source.slice(i))[0];
        push(num.indexOf(".") === -1 ? "int" : "float", num, col, num.length);
        i += num.length;
        continue;
      }

      if (/[A-Za-z_]/.test(ch)) {
        var word = /^[A-Za-z_][A-Za-z0-9_]*/.exec(source.slice(i))[0];
        push(KEYWORDS[word] ? "keyword" : "ident", word, col, word.length);
        i += word.length;
        continue;
      }

      var matched = null;
      for (var p = 0; p < PUNCT.length; p++) {
        if (source.slice(i, i + PUNCT[p].length) === PUNCT[p]) {
          matched = PUNCT[p];
          break;
        }
      }
      if (matched) {
        push("punct", matched, col, matched.length);
        i += matched.length;
        continue;
      }

      throw new JinError("unexpected character '" + ch + "' on line " + line);
    }

    tokens.push({ type: "eof", value: "", line: line, col: col });
    return tokens;
  }

  /* ------------------------------------------------------------- parser */

  function Parser(tokens) {
    this.tokens = tokens;
    this.pos = 0;
  }

  Parser.prototype.peek = function (offset) {
    return this.tokens[Math.min(this.pos + (offset || 0), this.tokens.length - 1)];
  };

  Parser.prototype.next = function () {
    return this.tokens[this.pos++];
  };

  Parser.prototype.at = function (type, value) {
    var t = this.peek();
    return t.type === type && (value === undefined || t.value === value);
  };

  Parser.prototype.accept = function (type, value) {
    if (this.at(type, value)) return this.next();
    return null;
  };

  Parser.prototype.expect = function (type, value) {
    var token = this.accept(type, value);
    if (!token) {
      throw new JinError(
        "expected " + (value || type) + " on line " + this.peek().line +
        ", got " + (this.peek().value || this.peek().type)
      );
    }
    return token;
  };

  Parser.prototype.skipSemis = function () {
    while (this.at("punct", ";")) this.next();
  };

  /* Program := Statement* */
  Parser.prototype.parseProgram = function () {
    var body = [];
    this.skipSemis();
    while (!this.at("eof")) {
      body.push(this.parseStatement());
      this.skipSemis();
    }
    return { type: "Program", body: body };
  };

  Parser.prototype.parseStatement = function () {
    var token = this.peek();

    if (token.type === "keyword") {
      switch (token.value) {
        case "let": return this.parseLet();
        case "if": return this.parseIf();
        case "while": return this.parseWhile();
        case "for": return this.parseFor();
        case "return": return this.parseReturn();
        case "break": this.next(); return { type: "Break" };
        case "continue": this.next(); return { type: "Continue" };
        case "fn": return this.parseFn(true);
        case "struct": return this.parseStruct();
        case "try": return this.parseTry();
        case "import":
          throw new JinError(
            "imports are not available in the browser playground — " +
            "run this one locally with the jin command."
          );
        default: break;
      }
    }

    var expr = this.parseExpression();
    if (this.at("punct", "=")) {
      this.next();
      return {
        type: "Assign",
        target: expr,
        value: this.parseExpression()
      };
    }
    return { type: "ExprStatement", expr: expr };
  };

  Parser.prototype.parseLet = function () {
    this.next(); // let
    var name = this.expect("ident").value;
    var annotation = null;
    if (this.accept("punct", ":")) {
      annotation = this.expect("ident").value;
    }
    var value = null;
    if (this.accept("punct", "=")) {
      value = this.parseExpression();
    }
    return {
      type: "Let",
      name: name,
      annotation: annotation,
      value: value
    };
  };

  Parser.prototype.parseIf = function () {
    this.next(); // if
    var test = this.parseExpression(true);
    this.expect("punct", "{");
    var consequent = this.parseBlock();
    var alternate = null;
    if (this.accept("keyword", "else")) {
      if (this.accept("keyword", "if")) {
        alternate = { type: "If", test: this.parseExpression(true), body: null, alt: null };
        this.expect("punct", "{");
        alternate.body = this.parseBlock();
      } else {
        this.expect("punct", "{");
        alternate = this.parseBlock();
      }
    }
    return { type: "If", test: test, body: consequent, alt: alternate };
  };

  Parser.prototype.parseWhile = function () {
    this.next();
    var test = this.parseExpression(true);
    this.expect("punct", "{");
    return { type: "While", test: test, body: this.parseBlock() };
  };

  Parser.prototype.parseFor = function () {
    this.next();
    var name = this.expect("ident").value;
    this.expect("keyword", "in");
    var iterable = this.parseExpression(true);
    this.expect("punct", "{");
    return { type: "For", name: name, iterable: iterable, body: this.parseBlock() };
  };

  Parser.prototype.parseReturn = function () {
    this.next();
    var value = null;
    if (!this.at("punct", ";") && !this.at("punct", "}") && !this.at("eof")) {
      var last = this.tokens[this.pos - 1];
      var next = this.peek();
      if (next.line === last.line) {
        value = this.parseExpression();
      }
    }
    return { type: "Return", value: value };
  };

  Parser.prototype.parseTry = function () {
    this.next(); // try
    this.expect("punct", "{");
    var body = this.parseBlock();
    this.expect("keyword", "catch");
    var name = this.expect("ident").value;
    this.expect("punct", "{");
    return { type: "Try", body: body, name: name, handler: this.parseBlock() };
  };

  Parser.prototype.parseStruct = function () {
    this.next(); // struct
    var name = this.expect("ident").value;
    this.expect("punct", "{");
    var fields = [];
    while (!this.at("punct", "}")) {
      var field = this.expect("ident").value;
      this.expect("punct", ":");
      var type = this.expect("ident").value;
      fields.push({ name: field, type: type });
      if (!this.accept("punct", ",")) break;
    }
    this.expect("punct", "}");
    return { type: "StructDecl", name: name, fields: fields };
  };

  Parser.prototype.parseFn = function (isStatement) {
    this.next(); // fn
    var receiver = null;
    var name = null;

    if (this.at("punct", "(")) {
      // fn (p: Point) method(...) — a method declaration
      this.next();
      var recvName = this.expect("ident").value;
      this.expect("punct", ":");
      var recvType = this.expect("ident").value;
      this.expect("punct", ")");
      receiver = { name: recvName, type: recvType };
      name = this.expect("ident").value;
    } else if (this.at("ident")) {
      name = this.next().value;
    }

    this.expect("punct", "(");
    var params = [];
    while (!this.at("punct", ")")) {
      var pname = this.expect("ident").value;
      var ptype = null;
      if (this.accept("punct", ":")) ptype = this.expect("ident").value;
      params.push({ name: pname, type: ptype });
      if (!this.accept("punct", ",")) break;
    }
    this.expect("punct", ")");

    var returnType = null;
    if (this.accept("punct", "->")) returnType = this.expect("ident").value;

    this.expect("punct", "{");
    var body = this.parseBlock();

    return {
      type: "FnDecl",
      name: name,
      receiver: receiver,
      params: params,
      returnType: returnType,
      body: body
    };
  };

  Parser.prototype.parseBlock = function () {
    var body = [];
    this.skipSemis();
    while (!this.at("punct", "}") && !this.at("eof")) {
      body.push(this.parseStatement());
      this.skipSemis();
    }
    this.expect("punct", "}");
    return body;
  };

  /* -------------------------------------------------------- expressions */

  Parser.prototype.parseExpression = function (noBrace) {
    return this.parseOr(noBrace);
  };

  Parser.prototype.parseOr = function (noBrace) {
    var left = this.parseAnd(noBrace);
    while (this.at("punct", "||") && this.sameLine()) {
      this.next();
      left = { type: "Logical", op: "||", left: left, right: this.parseAnd(noBrace) };
    }
    return left;
  };

  Parser.prototype.parseAnd = function (noBrace) {
    var left = this.parseComparison(noBrace);
    while (this.at("punct", "&&") && this.sameLine()) {
      this.next();
      left = { type: "Logical", op: "&&", left: left, right: this.parseComparison(noBrace) };
    }
    return left;
  };

  Parser.prototype.parseComparison = function (noBrace) {
    var left = this.parseAdditive(noBrace);
    while (
      this.at("punct", "==") || this.at("punct", "!=") ||
      this.at("punct", "<") || this.at("punct", ">") ||
      this.at("punct", "<=") || this.at("punct", ">=")
    ) {
      if (!this.sameLine()) break;
      var opToken = this.next();
      left = {
        type: "Binary", op: opToken.value, left: left,
        right: this.parseAdditive(noBrace),
        line: opToken.line, col: opToken.col
      };
    }
    return left;
  };

  Parser.prototype.parseAdditive = function (noBrace) {
    var left = this.parseMultiplicative(noBrace);
    while ((this.at("punct", "+") || this.at("punct", "-")) && this.sameLine()) {
      var opToken = this.next();
      left = {
        type: "Binary", op: opToken.value, left: left,
        right: this.parseMultiplicative(noBrace),
        line: opToken.line, col: opToken.col
      };
    }
    return left;
  };

  Parser.prototype.parseMultiplicative = function (noBrace) {
    var left = this.parseUnary(noBrace);
    while (
      (this.at("punct", "*") || this.at("punct", "/") || this.at("punct", "%")) &&
      this.sameLine()
    ) {
      var opToken = this.next();
      left = {
        type: "Binary", op: opToken.value, left: left,
        right: this.parseUnary(noBrace),
        line: opToken.line, col: opToken.col
      };
    }
    return left;
  };

  Parser.prototype.parseUnary = function (noBrace) {
    if (this.at("punct", "!") || this.at("punct", "-")) {
      var op = this.next().value;
      return { type: "Unary", op: op, operand: this.parseUnary(noBrace) };
    }
    return this.parsePostfix(noBrace);
  };

  Parser.prototype.parsePostfix = function (noBrace) {
    var start = this.peek();
    var expr = this.parsePrimary(noBrace);
    if (expr.line === undefined) {
      expr.line = start.line;
      expr.col = start.col;
    }
    for (;;) {
      if (this.at("punct", "(")) {
        var open = this.next();
        var args = [];
        while (!this.at("punct", ")")) {
          args.push(this.parseExpression());
          if (!this.accept("punct", ",")) break;
        }
        this.expect("punct", ")");
        expr = {
          type: "Call", callee: expr, args: args,
          line: open.line, col: open.col
        };
        continue;
      }
      if (this.at("punct", "[")) {
        var bracket = this.next();
        var index = this.parseExpression();
        this.expect("punct", "]");
        expr = {
          type: "Index", object: expr, index: index,
          line: bracket.line, col: bracket.col
        };
        continue;
      }
      if (this.at("punct", ".")) {
        var dot = this.next();
        var field = this.expect("ident").value;
        expr = {
          type: "Member", object: expr, field: field,
          line: dot.line, col: dot.col
        };
        continue;
      }
      break;
    }
    return expr;
  };

  Parser.prototype.parsePrimary = function (noBrace) {
    var token = this.peek();

    if (token.type === "int") { this.next(); return { type: "Literal", value: Int(parseInt(token.value, 10)) }; }
    if (token.type === "float") { this.next(); return { type: "Literal", value: Float(parseFloat(token.value)) }; }
    if (token.type === "string") {
      this.next();
      var parts = token.parts || [];
      if (parts.length === 0) {
        return { type: "Literal", value: Str("") };
      }
      if (parts.length === 1 && parts[0].kind === "text") {
        return { type: "Literal", value: Str(parts[0].value) };
      }
      var pieces = [];
      for (var p = 0; p < parts.length; p++) {
        if (parts[p].kind === "text") {
          pieces.push({ kind: "text", value: parts[p].value });
          continue;
        }
        var sub = new Parser(lex(parts[p].source));
        pieces.push({ kind: "expr", node: sub.parseExpression() });
      }
      return { type: "Interp", parts: pieces };
    }

    if (token.type === "keyword") {
      if (token.value === "true") { this.next(); return { type: "Literal", value: TRUE }; }
      if (token.value === "false") { this.next(); return { type: "Literal", value: FALSE }; }
      if (token.value === "nil") { this.next(); return { type: "Literal", value: NIL }; }
      if (token.value === "fn") {
        this.next();
        this.expect("punct", "(");
        var params = [];
        while (!this.at("punct", ")")) {
          var pname = this.expect("ident").value;
          var ptype = null;
          if (this.accept("punct", ":")) ptype = this.expect("ident").value;
          params.push({ name: pname, type: ptype });
          if (!this.accept("punct", ",")) break;
        }
        this.expect("punct", ")");
        this.expect("punct", "{");
        return { type: "FnExpr", params: params, body: this.parseBlock() };
      }
      if (token.value === "struct") {
        this.next();
        var sname = this.expect("ident").value;
        this.expect("punct", "{");
        var sfields = [];
        while (!this.at("punct", "}")) {
          var fname = this.expect("ident").value;
          this.expect("punct", ":");
          var ftype = this.expect("ident").value;
          sfields.push({ name: fname, type: ftype });
          if (!this.accept("punct", ",")) break;
        }
        this.expect("punct", "}");
        return { type: "StructExpr", name: sname, fields: sfields };
      }
    }

    if (token.type === "ident") {
      this.next();
      // A struct literal: Point{x: 1, y: 2}
      if (!noBrace && this.at("punct", "{")) {
        var save = this.pos;
        this.next();
        var named = [];
        var ok = true;
        while (!this.at("punct", "}")) {
          var key = this.accept("ident");
          if (!key) { ok = false; break; }
          this.expect("punct", ":");
          named.push({ name: key.value, value: this.parseExpression() });
          if (!this.accept("punct", ",")) break;
        }
        if (ok && this.at("punct", "}")) {
          this.next();
          return { type: "StructLiteral", name: token.value, fields: named };
        }
        this.pos = save;
      }
      return { type: "Ident", name: token.value };
    }

    if (token.type === "punct") {
      if (token.value === "(") {
        this.next();
        var inner = this.parseExpression();
        this.expect("punct", ")");
        return { type: "Group", expr: inner };
      }
      if (token.value === "[") {
        this.next();
        var items = [];
        while (!this.at("punct", "]")) {
          items.push(this.parseExpression());
          if (!this.accept("punct", ",")) break;
        }
        this.expect("punct", "]");
        return { type: "ArrayLit", items: items };
      }
      if (token.value === "{") {
        this.next();
        var pairs = [];
        while (!this.at("punct", "}")) {
          var keyToken = this.next();
          if (keyToken.type === "string") {
            var keyText = "";
            var keyParts = keyToken.parts || [];
            for (var q = 0; q < keyParts.length; q++) {
              if (keyParts[q].kind !== "text") {
                throw new JinError("a map key cannot be an expression");
              }
              keyText += keyParts[q].value;
            }
            var mapKey = Str(keyText);
          } else if (keyToken.type === "int") {
            var mapKey = Int(parseInt(keyToken.value, 10));
          } else if (keyToken.type === "keyword" && keyToken.value === "true") {
            var mapKey = TRUE;
          } else if (keyToken.type === "keyword" && keyToken.value === "false") {
            var mapKey = FALSE;
          } else {
            throw new JinError("bad map key on line " + keyToken.line);
          }
          this.expect("punct", ":");
          pairs.push([
            { type: "Literal", value: mapKey },
            this.parseExpression()
          ]);
          if (!this.accept("punct", ",")) break;
        }
        this.expect("punct", "}");
        return { type: "MapLit", pairs: pairs };
      }
    }

    throw new JinError(
      "unexpected " + (token.value || token.type) + " on line " + token.line
    );
  };

  /* Whether the upcoming token is on the same line as the last one. */
  Parser.prototype.sameLine = function () {
    if (this.pos === 0) return true;
    var previous = this.tokens[this.pos - 1];
    var current = this.peek();
    if (current.line === previous.line) return true;
    var openers = ["(", "[", "{", ",", "+", "-", "*", "/", "%", "&&", "||",
      "==", "!=", "<", ">", "<=", ">=", "="];
    return openers.indexOf(previous.value) !== -1;
  };

  /* -------------------------------------------------------- environments */

  function Env(parent) {
    this.vars = Object.create(null);
    this.parent = parent || null;
  }

  Env.prototype.get = function (name) {
    var env = this;
    while (env) {
      if (name in env.vars) return env.vars[name];
      env = env.parent;
    }
    throw new JinError("unknown name '" + name + "'");
  };

  Env.prototype.has = function (name) {
    var env = this;
    while (env) {
      if (name in env.vars) return true;
      env = env.parent;
    }
    return false;
  };

  Env.prototype.set = function (name, value) {
    var env = this;
    while (env) {
      if (name in env.vars) {
        env.vars[name] = value;
        return;
      }
      env = env.parent;
    }
    this.vars[name] = value;
  };

  Env.prototype.declare = function (name, value) {
    this.vars[name] = value;
  };

  /* ----------------------------------------------------------- built-ins */

  function makeBuiltins(output, interp) {
    var b = {};

    function def(name, arity, fn) {
      b[name] = new Builtin(name, fn, arity);
    }

    def("print", -1, function (args) {
      output.push(args.map(inspect).join(" "));
    });

    def("len", 1, function (args) {
      var v = args[0];
      if (isStr(v)) return Int(v.v.length);
      if (isArr(v)) return Int(v.v.length);
      if (isMap(v)) return Int(v.order.length);
      if (isRec(v)) return Int(Object.keys(v.f).length);
      throw new JinError("len() wants a string, array or map");
    });

    def("type", 1, function (args) {
      return Str(typeName(args[0]));
    });

    def("str", 1, function (args) {
      return Str(inspect(args[0]));
    });

    def("int", 1, function (args) {
      var v = args[0];
      if (isInt(v)) return v;
      if (isFloat(v)) return Int(Math.trunc(v.v));
      if (isStr(v)) {
        var n = parseInt(v.v.trim(), 10);
        if (isNaN(n)) throw new JinError("int() cannot read '" + v.v + "'");
        return Int(n);
      }
      if (isBool(v)) return Int(v.v ? 1 : 0);
      throw new JinError("int() cannot convert " + typeName(v));
    });

    def("float", 1, function (args) {
      var v = args[0];
      if (isFloat(v)) return v;
      if (isInt(v)) return Float(v.v);
      if (isStr(v)) {
        var n = parseFloat(v.v.trim());
        if (isNaN(n)) throw new JinError("float() cannot read '" + v.v + "'");
        return Float(n);
      }
      throw new JinError("float() cannot convert " + typeName(v));
    });

    def("range", -1, function (args) {
      if (args.length === 1) {
        var n = args[0];
        if (!isInt(n)) throw new JinError("range() wants an int");
        var one = [];
        for (var i = 0; i < n.v; i++) one.push(Int(i));
        return Arr(one);
      }
      if (args.length === 2) {
        var from = args[0], to = args[1];
        if (!isInt(from) || !isInt(to)) {
          throw new JinError("range() wants ints");
        }
        var two = [];
        for (var j = from.v; j < to.v; j++) two.push(Int(j));
        return Arr(two);
      }
      throw new JinError("range() wants one or two ints");
    });

    def("push", 2, function (args) {
      if (!isArr(args[0])) throw new JinError("push() wants an array");
      return Arr(args[0].v.concat([args[1]]));
    });

    def("upper", 1, function (args) {
      return Str(args[0].v.toUpperCase());
    });
    def("lower", 1, function (args) {
      return Str(args[0].v.toLowerCase());
    });
    def("trim", 1, function (args) {
      return Str(args[0].v.trim());
    });
    def("split", 2, function (args) {
      return Arr(args[0].v.split(args[1].v).map(Str));
    });
    def("join", 2, function (args) {
      if (!isArr(args[0])) throw new JinError("join() wants an array");
      if (!isStr(args[1])) throw new JinError("join() wants a string separator");
      var items = args[0].v.map(function (v) { return inspect(v); });
      return Str(items.join(args[1].v));
    });
    def("replace", 3, function (args) {
      return Str(args[0].v.split(args[1].v).join(args[2].v));
    });
    def("find", 2, function (args) {
      var index = args[0].v.indexOf(args[1].v);
      return Int(index === -1 ? -1 : index);
    });
    def("starts_with", 2, function (args) {
      return Bool(args[0].v.indexOf(args[1].v) === 0);
    });
    def("ends_with", 2, function (args) {
      var s = args[0].v, t = args[1].v;
      return Bool(s.length >= t.length && s.slice(s.length - t.length) === t);
    });
    def("repeat", 2, function (args) {
      var out = "";
      for (var i = 0; i < args[1].v; i++) out += args[0].v;
      return Str(out);
    });
    def("chars", 1, function (args) {
      return Arr(args[0].v.split("").map(Str));
    });
    def("lines", 1, function (args) {
      var text = args[0].v.replace(/\r\n/g, "\n");
      var parts = text.split("\n");
      if (parts.length && parts[parts.length - 1] === "") parts.pop();
      return Arr(parts.map(Str));
    });

    def("contains", 2, function (args) {
      var a = args[0], b = args[1];
      if (isStr(a)) return Bool(a.v.indexOf(b.v) !== -1);
      if (isArr(a)) {
        for (var i = 0; i < a.v.length; i++) if (equals(a.v[i], b)) return TRUE;
        return FALSE;
      }
      if (isMap(a)) return Bool(a.has(b));
      throw new JinError("contains() wants a string, array or map");
    });

    def("abs", 1, function (args) {
      var v = args[0];
      if (isInt(v)) return Int(Math.abs(v.v));
      if (isFloat(v)) return Float(Math.abs(v.v));
      throw new JinError("abs() wants a number");
    });
    def("min", -1, function (args) {
      return minMax(args, true);
    });
    def("max", -1, function (args) {
      return minMax(args, false);
    });
    def("floor", 1, function (args) {
      return Int(Math.floor(args[0].v));
    });
    def("ceil", 1, function (args) {
      return Int(Math.ceil(args[0].v));
    });
    def("round", 1, function (args) {
      return Int(Math.round(args[0].v));
    });
    def("sqrt", 1, function (args) {
      return Float(Math.sqrt(args[0].v));
    });
    def("pow", 2, function (args) {
      var r = Math.pow(args[0].v, args[1].v);
      return isInt(args[0]) && isInt(args[1]) ? Int(r) : Float(r);
    });
    def("random", 0, function () {
      return Float(Math.random());
    });

    def("has", 2, function (args) {
      if (!isMap(args[0])) throw new JinError("has() wants a map");
      return Bool(args[0].has(args[1]));
    });
    def("get", 3, function (args) {
      if (!isMap(args[0])) throw new JinError("get() wants a map");
      return args[0].has(args[1]) ? args[0].get(args[1]) : args[2];
    });
    def("delete", 2, function (args) {
      if (!isMap(args[0])) throw new JinError("delete() wants a map");
      args[0].del(args[1]);
      return NIL;
    });
    def("keys", 1, function (args) {
      return Arr(args[0].order.slice());
    });
    def("values", 1, function (args) {
      var map = args[0];
      return Arr(map.order.map(function (k) { return map.get(k); }));
    });

    def("sort", 1, function (args) {
      if (!isArr(args[0])) throw new JinError("sort() wants an array");
      var copy = args[0].v.slice();
      copy.sort(compareValues);
      return Arr(copy);
    });
    def("reverse", 1, function (args) {
      if (!isArr(args[0])) throw new JinError("reverse() wants an array");
      return Arr(args[0].v.slice().reverse());
    });
    def("slice", 3, function (args) {
      if (!isArr(args[0])) throw new JinError("slice() wants an array");
      var start = args[1].v, end = args[2].v, len = args[0].v.length;
      if (start < 0 || end > len || start > end) {
        throw new JinError(
          "slice() start " + start + " is out of range (length " + len + ")"
        );
      }
      return Arr(args[0].v.slice(start, end));
    });

    def("map", 2, function (args) {
      return Arr(args[0].v.map(function (v) { return interp.call(args[1], [v]); }));
    });
    def("filter", 2, function (args) {
      return Arr(
        args[0].v.filter(function (v) { return isTrue(interp.call(args[1], [v])); })
      );
    });
    def("reduce", 3, function (args) {
      var acc = args[2];
      for (var i = 0; i < args[0].v.length; i++) {
        acc = interp.call(args[1], [acc, args[0].v[i]]);
      }
      return acc;
    });
    def("any", 2, function (args) {
      for (var i = 0; i < args[0].v.length; i++) {
        if (isTrue(interp.call(args[1], [args[0].v[i]]))) return TRUE;
      }
      return FALSE;
    });
    def("all", 2, function (args) {
      for (var i = 0; i < args[0].v.length; i++) {
        if (!isTrue(interp.call(args[1], [args[0].v[i]]))) return FALSE;
      }
      return TRUE;
    });
    def("sort_by", 2, function (args) {
      // sort_by takes a comparator: fn(a, b) returns a negative number, zero
      // or a positive number, and the sort is stable.
      var copy = args[0].v.slice();
      var decorated = copy.map(function (v, i) { return { v: v, i: i }; });
      decorated.sort(function (a, b) {
        var result = interp.call(args[1], [a.v, b.v]);
        if (!isNum(result)) {
          throw new JinError("sort_by() comparator must return a number, got " + typeName(result));
        }
        return result.v !== 0 ? result.v : a.i - b.i;
      });
      return Arr(decorated.map(function (d) { return d.v; }));
    });
    def("find_index", 2, function (args) {
      for (var i = 0; i < args[0].v.length; i++) {
        if (isTrue(interp.call(args[1], [args[0].v[i]]))) return Int(i);
      }
      return Int(-1);
    });

    def("error", 1, function (args) {
      throw located(interp.callSite, args[0].v);
    });

    def("args", 0, function () {
      return Arr([]);
    });

    ["read_file", "write_file", "append_file", "file_exists", "delete_file",
      "input", "read_all_stdin"].forEach(function (name) {
      def(name, -1, function () {
        throw new JinError(
          name + "() is not available in the browser playground"
        );
      });
    });

    return b;
  }

  function minMax(args, wantMin) {
    if (args.length === 0) throw new JinError("min()/max() want at least one number");
    if (args.length === 1 && isArr(args[0])) {
      if (!args[0].v.length) {
        throw new JinError("min()/max() want at least one number");
      }
      var items = args[0].v;
      var smallest = items[0];
      for (var k = 1; k < items.length; k++) {
        if (compareValues(items[k], smallest) < 0) smallest = items[k];
      }
      return smallest;
    }
    var best = args[0];
    for (var i = 1; i < args.length; i++) {
      var c = compareValues(args[i], best);
      if (wantMin ? c < 0 : c > 0) best = args[i];
    }
    return best;
  }

  function compareValues(a, b) {
    if (isNum(a) && isNum(b)) {
      if (a.t !== b.t) {
        // Jin keeps int and float apart, but for ordering the numeric value
        // is what matters.
        return a.v < b.v ? -1 : a.v > b.v ? 1 : 0;
      }
      return a.v < b.v ? -1 : a.v > b.v ? 1 : 0;
    }
    if (isStr(a) && isStr(b)) {
      return a.v < b.v ? -1 : a.v > b.v ? 1 : 0;
    }
    if (isBool(a) && isBool(b)) {
      return a.v === b.v ? 0 : a.v ? 1 : -1;
    }
    throw new JinError("cannot compare " + typeName(a) + " with " + typeName(b));
  }

  function equals(a, b) {
    if (a.t !== b.t) return false;
    switch (a.t) {
      case "int": case "float": case "str": case "bool":
        return a.v === b.v;
      case "nil":
        return true;
      case "arr": {
        if (a.v.length !== b.v.length) return false;
        for (var i = 0; i < a.v.length; i++) {
          if (!equals(a.v[i], b.v[i])) return false;
        }
        return true;
      }
      case "map": {
        if (a.order.length !== b.order.length) return false;
        for (var j = 0; j < a.order.length; j++) {
          if (!a.has(b.order[j])) return false;
          if (!equals(a.get(a.order[j]), b.get(b.order[j]))) return false;
        }
        return true;
      }
      case "rec":
        return a.type === b.type && equals(Arr(Object.keys(a.f).map(function (k) { return a.f[k]; })),
          Arr(Object.keys(b.f).map(function (k) { return b.f[k]; })));
      default:
        return false;
    }
  }

  /* A runtime error, written the way Jin writes it: line, column, message. */
  function located(node, message) {
    if (node && node.line !== undefined) {
      return new JinError(node.line + ":" + node.col + ": " + message);
    }
    return new JinError(message);
  }

  function isTrue(value) {
    if (isBool(value)) return value.v;
    throw new JinError("condition must be a bool, got " + typeName(value));
  }

  /* ---------------------------------------------------------- evaluator */

  function ReturnSignal(value) { this.value = value; }
  function BreakSignal() {}
  function ContinueSignal() {}

  function Interpreter(globals, builtins) {
    this.globals = globals;
    this.builtins = builtins;
  }

  Interpreter.prototype.run = function (program) {
    this.hoist(program.body, this.globals);
    this.execBlock(program.body, this.globals);
  };

  /* Function declarations are visible before their own definition, so that
     two functions can call each other. */
  Interpreter.prototype.hoist = function (body, env) {
    for (var i = 0; i < body.length; i++) {
      var node = body[i];
      if (node.type === "FnDecl" && node.name && !node.receiver) {
        env.declare(node.name, new FnObject(node, env, node.name));
      }
      if (node.type === "FnDecl" && node.receiver) {
        var type = node.receiver.type;
        if (!env.has("__methods__")) env.declare("__methods__", new Map());
        var methods = env.get("__methods__");
        if (!methods.has(Str(type))) methods.set(Str(type), new Map());
        var byName = methods.get(Str(type));
        byName.set(Str(node.name), node);
      }
    }
  };

  Interpreter.prototype.execBlock = function (body, env) {
    var last;
    for (var i = 0; i < body.length; i++) {
      var value = this.exec(body[i], env);
      if (value !== undefined) last = value;
    }
    return last;
  };

  Interpreter.prototype.exec = function (node, env) {
    switch (node.type) {
      case "ExprStatement":
        return this.eval(node.expr, env);

      case "Assign":
        this.assign(node, env);
        return undefined;

      case "Let":
        var value = node.value ? this.eval(node.value, env) : NIL;
        if (node.annotation && node.annotation !== "any") {
          this.checkAnnotation(node.annotation, value, node.name);
        }
        env.declare(node.name, value);
        return;

      case "FnDecl":
        if (node.receiver) return; // hoisted
        env.declare(node.name, new FnObject(node, env, node.name));
        return;

      case "StructDecl":
        env.declare(node.name, { t: "structtype", name: node.name, fields: node.fields });
        return;

      case "If":
        if (isTrue(this.eval(node.test, env))) {
          return this.execBlock(node.body, new Env(env));
        } else if (node.alt) {
          if (node.alt.type === "If") {
            return this.exec(node.alt, env);
          }
          return this.execBlock(node.alt, new Env(env));
        }
        return undefined;

      case "While":
        while (isTrue(this.eval(node.test, env))) {
          try {
            this.execBlock(node.body, new Env(env));
          } catch (signal) {
            if (signal instanceof BreakSignal) break;
            if (signal instanceof ContinueSignal) continue;
            throw signal;
          }
        }
        return;

      case "For":
        var iterable = this.eval(node.iterable, env);
        var items = this.iterate(iterable);
        for (var i = 0; i < items.length; i++) {
          // A fresh environment each pass: every closure made in the body
          // keeps its own binding.
          var scope = new Env(env);
          scope.declare(node.name, items[i]);
          try {
            this.execBlock(node.body, scope);
          } catch (signal) {
            if (signal instanceof BreakSignal) break;
            if (signal instanceof ContinueSignal) continue;
            throw signal;
          }
        }
        return;

      case "Return":
        throw new ReturnSignal(node.value ? this.eval(node.value, env) : NIL);

      case "Break":
        throw new BreakSignal();

      case "Continue":
        throw new ContinueSignal();

      case "Try":
        try {
          this.execBlock(node.body, new Env(env));
        } catch (error) {
          if (error instanceof ReturnSignal || error instanceof BreakSignal ||
              error instanceof ContinueSignal) {
            throw error;
          }
          var scope = new Env(env);
          scope.declare(node.name, Str(error && error.message ? error.message : String(error)));
          this.execBlock(node.handler, scope);
        }
        return;

      default:
        throw new JinError("cannot run " + node.type);
    }
  };

  Interpreter.prototype.checkAnnotation = function (annotation, value, name) {
    var ok = false;
    switch (annotation) {
      case "int": ok = isInt(value); break;
      case "float": ok = isFloat(value); break;
      case "str": ok = isStr(value); break;
      case "bool": ok = isBool(value); break;
      case "nil": ok = isNil(value); break;
      case "array": ok = isArr(value); break;
      case "map": ok = isMap(value); break;
      case "any": ok = true; break;
      default: ok = true;
    }
    if (!ok) {
      throw new JinError(
        "cannot use " + typeName(value) + " where " + annotation +
        " is expected (for " + name + ")"
      );
    }
  };

  Interpreter.prototype.iterate = function (value) {
    if (isArr(value)) return value.v;
    if (isStr(value)) return value.v.split("").map(Str);
    if (isMap(value)) return value.order.slice();
    throw new JinError("cannot loop over " + typeName(value));
  };

  Interpreter.prototype.eval = function (node, env) {
    switch (node.type) {
      case "Literal":
        return node.value;

      case "Group":
        return this.eval(node.expr, env);

      case "Interp": {
        var out = "";
        for (var i = 0; i < node.parts.length; i++) {
          var piece = node.parts[i];
          if (piece.kind === "text") {
            out += piece.value;
          } else {
            out += inspect(this.eval(piece.node, env));
          }
        }
        return Str(out);
      }

      case "Assign":
        return this.assign(node, env);

      case "Ident": {
        if (env.has(node.name)) return env.get(node.name);
        if (this.builtins[node.name]) return this.builtins[node.name];
        throw new JinError("unknown name '" + node.name + "'");
      }

      case "ArrayLit": {
        var items = [];
        for (var i = 0; i < node.items.length; i++) {
          items.push(this.eval(node.items[i], env));
        }
        return Arr(items);
      }

      case "MapLit": {
        var map = new Map();
        for (var j = 0; j < node.pairs.length; j++) {
          map.set(this.eval(node.pairs[j][0], env), this.eval(node.pairs[j][1], env));
        }
        return map;
      }

      case "StructLiteral": {
        var type = env.get(node.name);
        if (!type || type.t !== "structtype") {
          throw new JinError("unknown record type '" + node.name + "'");
        }
        var fields = {};
        var names = Object.create(null);
        var k;
        for (k = 0; k < type.fields.length; k++) names[type.fields[k].name] = true;
        for (k = 0; k < node.fields.length; k++) {
          if (!names[node.fields[k].name]) {
            throw new JinError(
              "record " + node.name + " has no field '" + node.fields[k].name + "'"
            );
          }
          fields[node.fields[k].name] = this.eval(node.fields[k].value, env);
        }
        for (k = 0; k < type.fields.length; k++) {
          if (!(type.fields[k].name in fields)) {
            throw new JinError(
              "record " + node.name + " is missing field '" + type.fields[k].name + "'"
            );
          }
        }
        var order = [];
        for (var f = 0; f < type.fields.length; f++) {
          order.push(type.fields[f].name);
        }
        return Rec(node.name, fields, order);
      }

      case "FnExpr":
        return new FnObject(node, env);

      case "Unary": {
        var operand = this.eval(node.operand, env);
        if (node.op === "!") return Bool(!isTrue(operand));
        if (isInt(operand)) return Int(-operand.v);
        if (isFloat(operand)) return Float(-operand.v);
        throw new JinError("cannot negate " + typeName(operand));
      }

      case "Logical": {
        var left = this.eval(node.left, env);
        var leftValue = isTrue(left);
        if (node.op === "&&") {
          if (!leftValue) return FALSE;
          return Bool(isTrue(this.eval(node.right, env)));
        }
        if (leftValue) return TRUE;
        return Bool(isTrue(this.eval(node.right, env)));
      }

      case "Binary":
        return this.binary(node, env);

      case "Index": {
        var object = this.eval(node.object, env);
        var index = this.eval(node.index, env);
        if (isArr(object)) {
          if (!isInt(index)) {
            throw located(node, "array index must be an int");
          }
          if (index.v < 0 || index.v >= object.v.length) {
            throw located(
              node,
              "index " + index.v + " out of range (length " + object.v.length + ")"
            );
          }
          return object.v[index.v];
        }
        if (isStr(object)) {
          if (!isInt(index)) throw new JinError("string index must be an int");
          if (index.v < 0 || index.v >= object.v.length) {
            throw new JinError(
              "index " + index.v + " is out of range (length " + object.v.length + ")"
            );
          }
          return Str(object.v.charAt(index.v));
        }
        if (isMap(object)) {
          if (!object.has(index)) {
            throw located(
              node, "key " + repr(index) + " is not in the map"
            );
          }
          return object.get(index);
        }
        throw new JinError("cannot index " + typeName(object));
      }

      case "Member": {
        var target = this.eval(node.object, env);
        if (isRec(target)) {
          if (node.field in target.f) return target.f[node.field];
          // A bound method: p.shift
          var bound = this.findMethod(target, node.field);
          if (bound) return new Method(target, bound);
          throw located(
            node, target.type + " has no field " + JSON.stringify(node.field)
          );
        }
        var method = this.findMethod(target, node.field);
        if (method) return new Method(target, method);
        throw new JinError("no field '" + node.field + "' on " + typeName(target));
      }

      case "Call": {
        var callee = this.eval(node.callee, env);
        var args = [];
        for (var a = 0; a < node.args.length; a++) {
          args.push(this.eval(node.args[a], env));
        }
        var previousSite = this.callSite;
        this.callSite = node;
        try {
          return this.call(callee, args);
        } finally {
          this.callSite = previousSite;
        }
      }

      default:
        throw new JinError("cannot evaluate " + node.type);
    }
  };

  Interpreter.prototype.assign = function (node, env) {
    var value = this.eval(node.value, env);
    var target = node.target;

    if (target.type === "Ident") {
      if (!env.has(target.name)) {
        throw new JinError("unknown name '" + target.name + "'");
      }
      env.set(target.name, value);
      return value;
    }

    if (target.type === "Member") {
      var object = this.eval(target.object, env);
      if (!isRec(object)) {
        throw new JinError("cannot set a field on " + typeName(object));
      }
      if (!(target.field in object.f)) {
        throw new JinError(
          "record " + object.type + " has no field '" + target.field + "'"
        );
      }
      object.f[target.field] = value;
      return value;
    }

    if (target.type === "Index") {
      var container = this.eval(target.object, env);
      var index = this.eval(target.index, env);
      if (isArr(container)) {
        if (!isInt(index)) throw new JinError("array index must be an int");
        if (index.v < 0 || index.v >= container.v.length) {
          throw new JinError(
            "index " + index.v + " is out of range (length " + container.v.length + ")"
          );
        }
        container.v[index.v] = value;
        return value;
      }
      if (isMap(container)) {
        container.set(index, value);
        return value;
      }
      throw new JinError("cannot assign into " + typeName(container));
    }

    throw new JinError("cannot assign to this expression");
  };

  Interpreter.prototype.findMethod = function (target, name) {
    if (!isRec(target)) return null;
    var methods = this.globals.has("__methods__") ? this.globals.get("__methods__") : null;
    if (!methods) return null;
    var byName = methods.get(Str(target.type));
    if (!byName) return null;
    return byName.get(Str(name)) || null;
  };

  Interpreter.prototype.binary = function (node, env) {
    var left = this.eval(node.left, env);
    var right = this.eval(node.right, env);
    var op = node.op;

    if (op === "==") return Bool(equals(left, right));
    if (op === "!=") return Bool(!equals(left, right));

    if (op === "+" || op === "-" || op === "*" || op === "/" || op === "%") {
      if (isStr(left) && isStr(right) && op === "+") {
        return Str(left.v + right.v);
      }
      if (!isNum(left) || !isNum(right)) {
        throw new JinError(
          "cannot use '" + op + "' on " + typeName(left) + " and " + typeName(right)
        );
      }
      if (left.t !== right.t) {
        throw new JinError(
          "cannot use '" + op + "' on " + typeName(left) + " and " + typeName(right) +
          "; convert one explicitly with int() or float()"
        );
      }
      if (left.t === "int") {
        var a = left.v, c = right.v;
        if (op === "+") return Int(a + c);
        if (op === "-") return Int(a - c);
        if (op === "*") return Int(a * c);
        if (op === "/") {
          if (c === 0) throw located(node, "division by zero");
          return Int(Math.trunc(a / c));
        }
        if (op === "%") {
          if (c === 0) throw located(node, "division by zero");
          return Int(a % c);
        }
      }
      var x = left.v, y = right.v;
      if (op === "+") return Float(x + y);
      if (op === "-") return Float(x - y);
      if (op === "*") return Float(x * y);
      if (op === "/") {
        if (y === 0) throw located(node, "division by zero");
        return Float(x / y);
      }
      if (op === "%") {
        if (y === 0) throw located(node, "division by zero");
        return Float(x % y);
      }
    }

    if (op === "<" || op === ">" || op === "<=" || op === ">=") {
      var c = compareValues(left, right);
      if (op === "<") return Bool(c < 0);
      if (op === ">") return Bool(c > 0);
      if (op === "<=") return Bool(c <= 0);
      return Bool(c >= 0);
    }

    throw new JinError("unknown operator '" + op + "'");
  };

  Interpreter.prototype.call = function (callee, args) {
    if (callee.t === "builtin") {
      return callee.fn(args);
    }

    if (callee.t === "method") {
      var methodNode = callee.fn;
      var scope = new Env(this.globals);
      scope.declare(methodNode.receiver.name, callee.recv);
      return this.invoke(methodNode, scope, args);
    }

    if (callee.t === "fn") {
      return this.invoke(callee.node, new Env(callee.env), args, callee);
    }

    throw new JinError("cannot call " + typeName(callee));
  };

  Interpreter.prototype.invoke = function (node, scope, args, closure) {
    for (var i = 0; i < node.params.length; i++) {
      var value = i < args.length ? args[i] : NIL;
      if (node.params[i].type) {
        this.checkAnnotation(node.params[i].type, value, node.params[i].name);
      }
      scope.declare(node.params[i].name, value);
    }
    var last;
    try {
      this.hoist(node.body, scope);
      last = this.execBlock(node.body, scope);
    } catch (signal) {
      if (signal instanceof ReturnSignal) return signal.value;
      throw signal;
    }
    // A function whose last statement is an expression returns it.
    return last === undefined ? NIL : last;
  };

  /* ------------------------------------------------------------- public */

  function run(source) {
    var output = [];
    try {
      var tokens = lex(source);
      var parser = new Parser(tokens);
      var program = parser.parseProgram();
      var globals = new Env(null);
      var interpreter = new Interpreter(globals, {});
      interpreter.builtins = makeBuiltins(output, interpreter);
      interpreter.run(program);
      return { output: output.join("\n"), error: null };
    } catch (error) {
      var message = error && error.message ? error.message : String(error);
      var text = output.join("\n");
      return {
        output: text,
        error: text ? text + "\n" + message : message
      };
    }
  }

  global.Jin = { run: run };
})(window);
