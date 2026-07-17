# luna
The Luna Programming Language

At the time of writing, no implementation exists. Only a spec and some barebones tooling exists.

# A Taste

```js
import std.io;
import std.json;

const exitCode = ['success' => 0, 'usage' => 64];

const main = fn () use (io, argv): int! => {
  let arguments = args();                        // argv is a capability
  die("usage: ${arguments[0]} <config.json>") if (arguments.count() != 2);

  var fd = openFile(arguments[1] as path);      // file!: error propogates out of main
  defer close(&fd);

  let config = fromJson(readAll(fd) as json);    // table!: bad JSON propagates too

  match (config['server']) {
    ['host' => h: string, 'port' => p: int] => println("serving on $h:$p"),
    ['port' => p: int]                      => println("serving on localhost:$p"),
    undefined                               => die('config has no server section'),
    _                                       => die('unrecognized server shape'),
  };

  return exitCode.success;
};
```

# Syntax Highlighting (zed)
```bash
git clone https://github.com/lucas-streanga/luna
```

- Use `ctrl-shift-p` to open the Zed command palette.
- Run `zed: install extension` and choose `luna/tooling/zed-luna` as the folder.
- You're done!

# Directory Structure:
- `docs/` - the specification, one Markdown file per topic, grouped into subdirectories by the
  part of the language it targets (`types/`, `expressions/`, `declarations/`, `concurrency/`,
  `build/`, `std/`, `internals/`, `type-operations/`, `bindings/`, `overview/`, `examples/`).
  - `docs/index.md` is the authoritative map of every spec file and what it owns; start there.
  - This is the design itself, not user-facing docs — it is deliberately in-depth.
- `CHANGES.md` - the design-decision log: a numbered sequence of rulings, each resolving a
  contradiction or open question with its rationale and the files it swept.
- `tooling/` - syntax highlighting only (tree-sitter grammar, VSCode extension, Zed extension).
- `user-docs/` - user-facing documentation (planned; does not exist yet).
