window.JIN_EXAMPLES = {
  "hello": [
    'fn fib(n) {',
    '    if n < 2 { return n }',
    '    return fib(n - 1) + fib(n - 2)',
    '}',
    '',
    'let fibs = []',
    'for i in range(10) {',
    '    fibs = push(fibs, fib(i))',
    '}',
    'print("fib: {fibs}")'
  ].join("\n"),

  "records": [
    'struct Point { x: int, y: int }',
    '',
    'fn (p: Point) shift(dx: int, dy: int) -> Point {',
    '    p.x = p.x + dx',
    '    p.y = p.y + dy',
    '    return p',
    '}',
    '',
    'let p = Point{y: 20, x: 10}',
    'p.shift(5, -10)',
    'print(p)',
    'print(p.x, p.y)'
  ].join("\n"),

  "closures": [
    'fn counter() {',
    '    let n = 0',
    '    return fn() {',
    '        n = n + 1',
    '        return n',
    '    }',
    '}',
    '',
    'let next = counter()',
    'print(next(), next(), next())',
    '',
    'let printers = []',
    'for word in ["one", "two", "three"] {',
    '    printers = push(printers, fn() { return word })',
    '}',
    'for p in printers { print(p()) }'
  ].join("\n"),

  "maps": [
    'let server = {"host": "local", "port": 8080, "up": true}',
    'print(server["host"], server["port"])',
    '',
    'server["port"] = 9090',
    'server["tags"] = ["a", "b"]',
    'delete(server, "up")',
    '',
    'print(len(server), keys(server))',
    'print(has(server, "up"))',
    '',
    'let counts = {}',
    'for w in ["pear", "apple", "pear"] {',
    '    counts[w] = get(counts, w, 0) + 1',
    '}',
    'print(counts)'
  ].join("\n"),

  "errors": [
    'fn risky(n) {',
    '    if n == 0 { error("n must not be zero") }',
    '    return 100 / n',
    '}',
    '',
    'try {',
    '    print(risky(0))',
    '} catch e {',
    '    print("failed: {e}")',
    '}',
    '',
    'try {',
    '    print([1, 2][9])',
    '} catch e {',
    '    print("failed: {e}")',
    '}'
  ].join("\n"),

  "functional": [
    'let words = ["pear", "apple", "pear", "fig"]',
    '',
    'print(map(words, fn(w) { upper(w) }))',
    'print(filter(words, fn(w) { len(w) > 3 }))',
    'print(reduce(words, fn(total, w) { total + len(w) }, 0))',
    'print(any(words, fn(w) { w == "fig" }))',
    'print(all(words, fn(w) { len(w) > 2 }))',
    'print(sort_by(words, fn(a, b) { len(a) - len(b) }))'
  ].join("\n")
};
