# Testing

Tests are declarations (tests spec): string-literal names, zero parameters, implicit
`undefined!`, failure is any throw. `luna -t` runs them in parallel tasks; `luna -t -c
prog` gates the build on them (compiler §0.1).

```
import std.io;
import std.json;

const parsePort = fn (raw: string): int! => {
  let n = raw.toDouble();                 // double!: malformed input propagates
  let port = n as int;                    // panics if fractional: misuse of this fn
  throw error("port out of range: $port") if (port < 1 || port > 65535);
  return port;
};

test 'parsePort accepts a normal port' {
  throw error('wrong value') if (parsePort("8080") != 8080);
}

test 'parsePort rejects out-of-range' {
  let r = try parsePort("70000");         // r : int | error (errors §8)
  throw error('expected an error') if (!(r is error));
}

test 'round-trips a config file' use (io) {
  var fd = openFile('fixtures/config.json' as path);
  defer close(&fd);
  let cfg = fromJson(readAll(fd) as json);
  throw error('missing port') if (!(cfg is table));
}
```

What it exercises:

- **Pass by returning, fail by throwing** (tests §2): both declarable errors and panics
  are collected; the third test failing to open its fixture is a real failure report, not
  a crash.
- **`try` as recovery** (errors §8): the second test *wants* the error as a value, so it
  recovers to the union and asserts on `is error`, no flow-narrowing, the explicit test.
- **Capabilities on tests** (tests §1): the io test declares `use (io)` and the runner is
  its granting entry point; invoked through `std.test`'s table instead, the call checks
  the caller's grant (capabilities §3.1, tests §4).
- **Isolation for free** (tests §3): the three run as parallel tasks and provably cannot
  race on Luna state; the fixture file is the kind of external resource the temp-path
  deferral (tests §6) is about.
