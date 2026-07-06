# Serialization: attributes in, specialized writer out

The flagship comptime pipeline (attributes §4, std.json): field attributes drive a
compile-time generator that returns a plain runtime serializer with the tags baked in.

```
import std.io;
import std.json;

const jsonTag = attribute ['tag' => string = ''];

const User = [
  #[jsonTag('user_name')] 'name' => string,
  #[jsonTag('user_age')]  'age'  => int,
];

const writeUser = toJson(comptype User);       // generated ONCE, at compile time

const main = fn () use (io): int! => {
  let alice = ['name' => 'Alice', 'age' => 30];

  let doc: json = writeUser(alice);            // runtime: no reflection, tags compiled in
  println("$doc");                             // {"user_name":"Alice","user_age":30}

  println("${toJsonDynamic(alice)}");          // {"name":"Alice","age":30}: tags erased,
                                               // the structural walk serializes the VALUE
  return 0;
};
```

What it exercises:

- **`comptype`** (reflection §3.2): `comptype User` reads the declaration descriptor off
  the value's comptime provenance, the only way attributes are reachable, since they have
  no runtime existence (attributes §1) and never perturb the type (`@alice == @User`'s
  value regardless of tags).
- **The generator pattern** (attributes §4): `toJson` extracts the field/tag pairs into
  plain data and returns `fn (any): json`, no dependent types; capturing the descriptor
  itself would not compile (`comptype` confinement, compiler §6).
- **The split API, on display** (std.json §2): the same value, two different documents,
  `toJson` serializes the *declaration* (tags honored), `toJsonDynamic` serializes the
  *value* (keys as they are). One function doing both would be phase-divergent, which
  folding forbids (functions §5.5).
- **`json` is a type** (std.json §1): `doc: json` carries the validated commitment; an
  embedder could splice it verbatim, the type is the escaping decision.
