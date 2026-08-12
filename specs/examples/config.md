# Configuration you can trust

Every service loads a config file, and every service then re-checks the same three fields
in four places because the loader handed back a bare table. Here the loader's return type
*is* the guarantee: a `@settings` exists only if its host is a hostname and its port is a
port, so nothing downstream re-validates.

```luna
import std.io;
import std.json;
import std.process;
import { path } from std.filesystem;

const port     = constraint p: int where p >= 1 where p <= 65535;
const poolSize = constraint n: int where n >= 1 where n <= 1024;
const hostname = constraint h: string where !h.isEmpty() where !h.contains(' ');

const logLevel = enum { debug, info, warn, fatal };

const settings = proto {
  const get host: hostname;              // required: no default, one binding per application
  const get port: port;
  let get workers: poolSize = 4;         // defaulted, per-table, overridable at apply
  let get level: logLevel = {info};
};

export const loadSettings = fn (raw: json): @settings! => {
  let doc = fromJson(raw);

  let host = doc['host'] ??? 'localhost';
  throw error("not a hostname: ${host}") if (!(host is hostname));

  let p = doc['port'] ??? 8080;
  throw error("port out of range: ${p}") if (!(p is port));

  let workers = doc['workers'] ??? 4;
  throw error("workers out of range: ${workers}") if (!(workers is poolSize));

  return [] apply settings(host: host as hostname, port: p as port, workers: workers as poolSize);
};

const main = fn () use (io, argv): int! => {
  let arguments = args();
  die("usage: ${arguments[0]} <config.json>") if (arguments.count() != 2);

  var fd = openFile(arguments[1] as path);
  defer close(fd);

  let cfg = loadSettings(readAll(fd) as json);
  println("serving ${cfg->host}:${cfg->port} — ${cfg->workers} workers, level ${cfg->level}");
  return 0;
};

test 'defaults fill in what the document omits' {
  let cfg = loadSettings('{"host":"db.internal"}' as json);
  throw error('wrong default') if (cfg->workers != 4);
}

test 'an out-of-range port is an error, not a value' {
  let r = try loadSettings('{"host":"db.internal","port":70000}' as json);
  throw error('expected an error') if (!(r is error));
}
```

What it exercises:

- **A constraint is a match arm with the body dropped** (constraints §1): `p: int where …`
  is the same typed binder as a parameter, and the multi-clause form is a conjunction that
  *reports which clause failed*. The name is the API — `typeName` says `"port"`, and a
  failing check cites it — which is why the anonymous inline form was rejected (§1.1).
- **`is` for the report, `as` for the value** (as §1): a config error should tell the
  operator what is wrong, so the check is `is` plus a `throw` carrying the offending value;
  the `as` that follows cannot fail. Reaching for `as` first would be a `typeError` panic —
  correct for a programming error, wrong for a bad config file.
- **`is` against a constraint runs the predicate over the base** (is §2, constraints §7):
  `8080 is port` is `true` for any `int` in range, not only for a value already carrying the
  `port` typeid. That is what makes this loader work at all — everything `fromJson` produces
  is a plain `int` or `string`, and the boundary is where it acquires a constrained type.
- **The proto block's mutability keywords are the ordinary ones** (protocols §2, §2.1):
  `const get host` with no default is per-table and bound once at apply, which is the
  record-field case; `let get workers = 4` is defaulted and so may be overridden by an
  initializer. A `const` *with* a default would be definition-fixed — uniform across every
  table, and not initializable — which is the wrong thing for a setting.
- **Application runs no user code** (protocols §4): `[] apply settings(host: …)` is typed
  data installed by machinery, atomic, with no constructor and no partially-initialized
  value. Validation that needs to *decide* something lives in the factory, which is an
  ordinary function returning `@settings!` (§4.5) — its errorability is its own and is
  declared in its type.
- **`???` coalesces absence and `null` together** (coalescing): a missing JSON key yields
  `undefined` and a `"port": null` yields `null`; a document is allowed to spell "not set"
  either way, so both fall to the default. Plain `??` would take the `null` as a value and
  fail the `is port` check with a confusing message.
- **`json` is the boundary type** (std.json §1, §3): `readAll(fd) as json` validates once,
  where the string becomes JSON, and `fromJson` trusts it thereafter. `loadSettings` takes
  `json` rather than `string` for the same reason its result is `@settings` rather than
  `table` — the type carries the check that was already paid for.
- **`fromJson` is pure and comptime-eligible** (std.json §3), so the tests' literal
  documents are parsed at build time; only `main`'s file read is a runtime cost.
- **Tests are declarations** (tests §1, §2): pass by returning, fail by throwing. The second
  test *wants* the error as a value, so it uses `try` and asserts on `is error` — the
  explicit test, with no flow narrowing to accidentally rely on.
- **A variant literal is target-typed** (enum §3.2): `{info}` needs no qualification as a
  member default, because the declared type `logLevel` is the target. Note the members:
  `error` is a keyword, so an enum cannot have a variant of that name — `fatal` is not a
  stylistic choice.
