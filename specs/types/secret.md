# Secret

A `secret` is a value marked sensitive: a credential, token, password, or key that must not
appear in logs, error messages, stack traces, or interpolated output. It is its **own
distinct type**, and its defining behavior is that it **redacts itself everywhere it would
be displayed** and can be unwrapped only through a deliberate, visible `reveal`. So a secret
is protected by default and exposed only on purpose.

`secret` wraps a **sensitive payload**: a `string`, a `bytes` (binary keys and blobs), or a
`table` — the last for data whose **structure itself** is the disclosure, an error's
stacktrace being the motivating case (R111, errors §2.1). It does **not** wrap arbitrary
values; there is no secret function, command, or regex (§6). The payload set is small and
concrete: sensitive text, bytes, or structure.

---

## 1. Why a distinct type, not a flagged string

A secret must not be usable as an ordinary string, because if it were, it would leak through
every string operation that did not think to preserve its sensitivity. The protection has to
be enforced by the type system, not left to each operation to remember. So `secret` is a
**distinct type**: a `secret` is not a `string`, and cannot be passed where a `string` is
expected, concatenated as a string, or displayed as one. The only way to obtain the
underlying string is `reveal` (§4), which is deliberate and greppable.

This is the opposite of an advisory "sensitive" flag on a string (as some frameworks do): a
flag leaves the value typed as a string, so it leaks the moment any operation drops the flag.
A distinct type cannot leak that way, because the type system refuses to treat it as a
string at all.

---

## 2. Representation: secretness is in the `typeid`, like error-ness

Secretness is **type-determining**: a secret string's type is `secret`, not `string`. Because
it is type-determining, it lives in the value's **`typeid`**, not in an `lval` flag, exactly
the treatment error-ness gets (value-representation §2.1, §3.1). This is deliberate and follows
the value-representation discipline: **type-determining properties belong in the `typeid`, never
in the flag byte**, because a flag could drift out of sync with the type it is supposed to
determine. `secret` is a genuine type, so "is this value secret?" is a **subtype check**
(`currentType <: secret`), an O(1) `typeinfo` lookup (value-representation §4.2), not a bit
test.

Contrast the two axes value-representation already distinguishes:

- **`isNull`** is a genuine per-value **flag** that does **not** change the type: a null string
  is still a `string`. Nullability is a dynamic state on an unchanged type, so it rides in the
  flag byte.
- **Secretness**, like error-ness, **does** change the type: a secret string is a `secret`, a
  distinct type. So, like error-ness, it is read from the `typeid` (a subtype test), not stored
  as a flag. Storing it as a flag would denormalize the type and invite the flag disagreeing
  with the id, the exact problem value-representation §2.1 avoids.

So `isSecret` is the O(1) subtype test `currentType <: secret`, and the type-level distinction
is what makes a secret un-leakable: the type system refuses to treat a `secret` as a `string`
(§1), and that refusal is carried in the type, not in a mutable bit.

---

## 3. Construction: `as secret`, explicit

A value is wrapped as secret with the **explicit `as secret` coercion**, so that marking a
value sensitive is always a visible, greppable act:

```
let token = "ghp_xxxxxxxxxxxx" as secret;    // token : secret (of a string)
let pw     = userInput as secret;             // wrap a runtime string
```

Wrapping a payload kind (`string`, `bytes`, or `table`) is a **total, free** coercion: any
value of a payload kind is trivially a valid secret (a `table` payload is a COW snapshot,
immutable by unreachability), so the coercion can never fail. The reverse is **not** free: a
`secret` does not coerce back to a `string`, only `reveal` (§5) extracts it. So the two
directions are asymmetric by design, wrapping is free and explicit (`as secret`), unwrapping
is deliberate and explicit (`reveal`), and neither happens implicitly.

Wrapping is **explicit-only**: there is no implicit `string -> secret` coercion. A bare
`string` never becomes a `secret` by being passed where a `secret` is expected; you must
write `as secret`. This is what `as` is for, explicit coercion, and it keeps "where are
secrets created" answerable by searching for `as secret`. Silent wrapping, even though it
would be fail-safe, would hide that, and auditability is the point.

### 3.1 A secret carries its payload kind

A secret's payload is one of three kinds — and that is ordinary **union** knowledge, not
a type parameter: `reveal` returns `string | bytes | table` (§5, R113), narrowed like any
union (`as`, `match`). Luna has no generics, and needs none here. (An earlier design
carried a static payload-kind tag with one extractor per kind — `revealBytes`,
`revealTable`, R111 — traded in by R113 for the one union-returning `reveal`: less static
precision, full uniformity, the "unions instead" doctrine.)

`secret` wraps `string`, `bytes`, and `table`, and nothing else (§6).

---

### 3.2 Gated construction: `secret(...)` (R79)

Beyond `as secret` (the default form), the **constructor** attaches gates:

```
const secret = fn (raw: string|bytes, ...gates: type): secret

const a = raw as secret;                     // ≡ secret(raw): gated by the default, [@reveal]
const b = secret(raw, @dbCred);              // gated by dbCred
const c = secret(raw, @dbCred, @prodAccess); // gated by BOTH (AND, §5)
```

- Each `gates` element is a **capability's typeid** (`@dbCred`), pure data, never the token
  itself, which stays confined (capabilities §3.1); a non-capability `type` **panics** at
  construction (compile error where statically evident).
- **Zero gates means the default set `[@reveal]`**, and there is deliberately no spelling
  for an *empty* set: an ungated secret is a contradiction.
- **Wrapping requires no capability**: hiding data hurts nobody. The gate set rides the
  **value** (like a function's requirement set, capabilities §3.1), not the typeid; all
  secrets are type `secret`.
- **Re-gating is reveal-then-rewrap** (`secret(reveal(s), @newGate)`), so changing a
  secret's gates requires authority over its current ones, by construction.

### 3.3 Naming a gate in a signature: the constraint idiom (R111)

Gates ride values (§3.2); a **signature** names them through an ordinary constraint — the
`json` pattern (json §1), here over the immutable base `secret`, so the predicate runs
**once, at entry**, and never again:

```
export const dbCred   = capability;
export const dbSecret = constraint s: secret where gatesOf(s).exists(@dbCred);

const connect = fn (cred: dbSecret) use (dbCred): conn! => {
  let raw = reveal(cred);      // gate ⊆ frame grant: statically discharged here
  ...
};
```

- **No generics.** The "parameterization over a capability" is a named type, which is how
  Luna spells every refinement. `@dbCred` in the predicate is the capability's typeid
  (pure data, §3.2), and membership is the catalogue's `exists`
  (iterable-functions §2.3) — one typeid compare per gate.
- **The signature tells the whole story**: `use (dbCred)` declares the *authority*,
  `cred: dbSecret` declares the *material*, and `reveal` joins them at the effect site
  with the one requirement-⊆-grant test that already exists (§5). Through a
  constraint-typed parameter inside a matching `use` frame, the compiler can prove the
  gate check passes and **elide it** (constraints §9.5); through bare `secret`, the
  runtime check stands.
- **The convention**: a module that exports a capability exports its secret constraint
  beside it — one line, the same convention as capabilities-are-consts.

## 4. Redaction: secret hides itself everywhere it displays

The core behavior: a `secret` renders as a redaction marker, never its contents, on **every**
display path, automatically. Because these paths all route through the type's display
behavior, a secret cannot accidentally stringify into the clear anywhere:

- **`toString`** yields `<secret>`, never the payload. Since interpolation and
  string coercion go through `toString`, `"token is $token"` yields `"token is <secret>"`.
  The placeholder names *what kind of thing* was concealed (R113): a reader of output knows
  to go look for a gate, not to wonder which redaction policy fired.
- **`debugJson`** of a command (command spec §5.2) renders a secret argument as `<secret>`.
- **Error messages, stack traces, and any display** show `<secret>`, because they route
  through the same display path.
- **`toJson`** renders every secret — string, bytes, or table payload alike — as the string
  `'<secret>'` (json §2.1, R113): serialization is a display path, and the placeholder is
  the secret behaving as designed. Lossy by design; revealed serialization is a separate,
  deliberate, delegated act (json §2.1).

This is what "safe by default" means for secrets: you do not have to remember to redact; the
value redacts itself wherever it goes, and only `reveal` (§5) exposes it.

---

## 5. `reveal`: the sole, deliberate exposure

The underlying value is obtained only through `reveal`:

```
fn reveal(s: secret): string | bytes | table   // the payload; narrow with `as` or `match` (R113)
fn canReveal(s: secret): bool                  // gate set ⊆ frame grant — the probe form (R113)
fn gatesOf(s: secret): list                    // the gate set, as typeids (elements are type values)
```

- **`reveal` checks the secret's gate set against the executing frame's grant** (R79):
  every gate in `gatesOf(s)` must be held (AND), **panic** on any shortfall. This is the
  secrets application of the one check the runtime already runs, requirement-set ⊆
  frame-grant, the same one-word bitmask test that guards value-mediated calls and `spawn`
  (capabilities §3.1), sourced from the secret instead of a function value. The check sits
  at the **effect site**, in the frame doing the revealing, so there is nothing to cache
  and nothing to smuggle: a gated secret can travel anywhere as inert data, and the
  laundering theorem extends verbatim, every actual reveal of a `@dbCred`-gated secret
  occurs under a declared `use (dbCred)` on the executing path.
- **The audit is now per-gate, which is the point**: grep `use (dbCred)` and you have the
  complete list of functions that can see *that* secret, not the list that can see
  everything. `use (reveal)` is demoted from skeleton key to the **default gate's** key: it
  opens `as secret` / `secret(raw)` values and nothing gated tighter. A function with no
  relevant grant **cannot** reveal a secret it holds, so "this code does not expose this
  secret" is still read off signatures, just precisely now.
- **`reveal` is the only way** to get a payload out — one extractor, a **union return**
  (R113, superseding R111's `revealBytes` / `revealTable` days after they landed: the
  "no overloading; unions instead" doctrine applied to reveal itself). The payload's kind
  is ordinary union typing, narrowed with `as string` (checked) or `match` (total). Named
  loudly, so every exposure is a visible `reveal` in the source; reached by call or UFCS:
  `reveal(token)` or `token.reveal()` (both still require the gates).
- **`canReveal` is the probe form** to `reveal`'s assertion form — the hard/soft pairing
  the language uses everywhere (`.` / `?.`, `->` / `?->`): the same gate ⊆ grant test,
  returned instead of enforced. The render-or-redact idiom:
  `canReveal(s) ? reveal(s) : '<secret>'`, panic-free over mixed-gate structures.
- **The `reveal*` naming convention** (R113): a capability whose *purpose* is revelation
  is `reveal*`-prefixed — `reveal` (the default gate), `revealStackTrace` (errors §2.1),
  and any future kin — so `grep "use (reveal"` returns **every revelation authority** in
  a program, declarations and delegations alike (capabilities §5.2). General capabilities
  (`dbCred`) may still serve as gates; the prefix marks purpose-built revelation
  authority, not the only gating spelling.

### 5.1 Reveal is concentrated at infrastructure boundaries

Revealing should be **rare in ordinary code and concentrated at the boundaries where a raw
value genuinely crosses out of the program.** The intended pattern:

- User code **wraps** a credential with `... as secret` and passes the `secret` around. It is
  redacted in every log, error, and interpolation automatically (§4), so it travels safely.
- Infrastructure **reveals** it once, at the boundary where the real value is actually
  needed. `exec`, for instance, reveals a secret argument internally, right before passing
  it to the spawned process (exec spec), so the raw credential exists only at the syscall
  boundary and never in user-visible output.

So the common path is wrap-and-pass (reveal-free in user code), and revealing lives in a few
audited infrastructure sites. This keeps raw secret values out of application logic and
confines exposure to controlled, reviewable points.

---

## 6. Scope: what secret does and does not wrap

- **Wraps `string`, `bytes`, and `table`** (R111): text, raw bytes, and structure.
- **Does not wrap `any`.** There is no real use for a secret function, secret command, or
  secret regex; admitting arbitrary payloads adds nonsense cases for no benefit. Because
  the payload set is small and its kind is carried in the type (§3.1), the extractors
  return a concrete type rather than `any`, and the right extractor is checked at compile
  time, which is why revealing needs no coercion.
- **The structure-vs-leaves line** decides which of two shapes sensitive compound data
  takes. **Wrap the leaves** when only the values are sensitive: a credentials object is
  an ordinary table whose sensitive leaf values are secret
  (`['host' => "db.example", 'password' => pw as secret]`), so the non-sensitive
  structure stays readable and secrecy is localized. **Wrap the table** when the
  *structure itself* is the disclosure — an error's stacktrace, whose frames leak paths
  and internal shape, is the motivating case (errors §2.1). A whole-table secret is the
  exception, not the default; reaching for one should mean the shape is the secret.

### 6.1 A secret cannot be a table key

A `secret` cannot be a table key. This is not a limitation but a **consequence**: a key must
be visible to be matched and iterated, which is the exact opposite of a secret, which must be
hidden. A value that is both "the thing you look up by" and "the thing you cannot read" is a
contradiction. It also already falls out of the key-type rule (keys are `string` or `int`,
tables spec); a `secret` is neither, so it is excluded mechanically.

The real need behind "secret keys" is served without them: sensitive data goes in table
**values** (with ordinary public keys), and using a secret as a lookup handle means
`reveal`ing it (you are using its value to match, which is an honest exposure). Redacting keys
in logs, if ever needed, is a display concern (do not log them, or hash them), not a reason
for a secret key.

---

## 7. Open questions

- **`reveal` as a keyword:** `reveal` requires the secret's gates
  (capabilities spec, R79), which is what makes non-revelation a guarantee. What remains open is
  whether `reveal` should *also* be a keyword (so it cannot be shadowed or aliased at all),
  on top of the capability gate; the capability requirement already prevents laundering
  (it rides in the type), so this is a minor additional hardening, not a necessity.
- **Mark-preserving operations:** whether a few operations (notably concatenation) may
  combine secrets into a new secret without revealing (`a . b` staying secret for secret
  operands), so credentials can be composed without exposure, kept minimal to limit leak
  vectors.
- **Equality and comparison:** whether two secrets may be compared for equality without
  revealing (a constant-time compare on the payloads), which is a common need for token
  checking and should avoid `reveal`ing both sides into the clear.
