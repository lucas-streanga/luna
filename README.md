# luna
The Luna Programming Language

At the time of writing, no implementation exists. Only a spec and some barebones tooling exists.

# A taste

```js
import std.io;
import std.json;

const exitCode = ['success' => 0, 'usage' => 64];

const main = fn () use (io, argv): int! => {
  let arguments = args();                        // argv is a capability (capabilities §9)
  die('usage: taste <config.json>') if (arguments.count() != 1);

  var fd = openFile(arguments[0] as path);       // file!: inside this fn!, failure propagates
  defer close(&fd);

  let config = fromJson(readAll(fd) as json);    // table!: malformed JSON propagates too

  match (config['server']) {
    ['host' => string h, 'port' => int p] => println("serving on http://$h:$p"),
    ['port' => int p]                     => println("serving on http://localhost:$p"),
    undefined => die('config has no server section'),
    _         => die('unrecognized server shape'),
  };

  return exitCode.success;
};
```

# Syntax highlighting (zed)
```bash
git clone https://lucas-streanga/luna
```

- Use `ctrl-shift-p` to open the Zed command palette.
- Run `zed: install extension` and choose `luna/tooling/zed-luna` as the folder.
- You're done!

# Directory structure:
- `spec/` - contains the specification of the luna language, in depth.
  - This is not intended to be user-facing, as it's very in-depth.
  - The directory is segmented by the part of the language it targets. `build`, `internals`, `std`, etc.
- `user-docs/` - user facing documentation (planned).
  - `user-docs/quick-start` - Quick start guide (planned).
