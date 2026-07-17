# Secret

A `secret` is a value marked sensitive: a credential, token, password, or key that must not
appear in logs, error messages, stack traces, or interpolated output. It is its **own
distinct type**, and its defining behavior is that it **redacts itself everywhere it would
be displayed** and can be unwrapped only through a deliberate, visible `reveal`. So a secret
is protected by default and exposed only on purpose.

`secret` wraps a **sensitive payload**: a `string` today, and `bytes` once that type exists
(binary keys and blobs). It does **not** wrap arbitrary values; there is no meaningful
"secret table" or "secret function" (§6). The payload set is small and concrete: sensitive
text or bytes.

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

`string -> secret` is a **total, free** coercion: like `int -> string`, any string is
trivially a valid secret, so the coercion can never fail. The reverse is **not** free: a
`secret` does not coerce back to a `string`, only `reveal` (§5) extracts it. So the two
directions are asymmetric by design, wrapping is free and explicit (`as secret`), unwrapping
is deliberate and explicit (`reveal`), and neither happens implicitly.

Wrapping is **explicit-only**: there is no implicit `string -> secret` coercion. A bare
`string` never becomes a `secret` by being passed where a `secret` is expected; you must
write `as secret`. This is what `as` is for, explicit coercion, and it keeps "where are
secrets created" answerable by searching for `as secret`. Silent wrapping, even though it
would be fail-safe, would hide that, and auditability is the point.

### 3.1 A secret carries its payload kind

A `secret` records **which payload kind it holds**, a secret *string* or a secret *bytes*,
as part of its type. This is a two-way tag (string or bytes), not a type parameter; Luna has
no generics, and this is not one. It exists so that extraction is **statically checked**:
`reveal` (string) on a secret-bytes, or `revealBytes` on a secret-string, is a **compile
error**, not a runtime surprise (§5). Because there are only two payload kinds, carrying this
tag is cheap and needs no general parameterization.

`secret` wraps a `string` (now) and `bytes` (deferred, when that type exists), and nothing
else (§6).

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

## 4. Redaction: secret hides itself everywhere it displays

The core behavior: a `secret` renders as a redaction marker, never its contents, on **every**
display path, automatically. Because these paths all route through the type's display
behavior, a secret cannot accidentally stringify into the clear anywhere:

- **`toString`** yields `<redacted>` (or similar), never the payload. Since interpolation and
  string coercion go through `toString`, `"token is $token"` yields `"token is <redacted>"`.
- **`debugJson`** of a command (command spec §5.2) renders a secret argument as `<redacted>`.
- **Error messages, stack traces, and any display** show `<redacted>`, because they route
  through the same display path.

This is what "safe by default" means for secrets: you do not have to remember to redact; the
value redacts itself wherever it goes, and only `reveal` (§5) exposes it.

---

## 5. `reveal`: the sole, deliberate exposure

The underlying value is obtained only through `reveal` (for a secret string) or `revealBytes`
(for a secret bytes):

```
fn reveal(s: secret): string          // the underlying string; compile error on a secret-bytes
fn revealBytes(s: secret): bytes      // the underlying bytes;  compile error on a secret-string (deferred)
fn gatesOf(s: secret): list           // the gate set, as typeids: check before revealing
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
- **`reveal` / `revealBytes` are the only way** to get a payload out. They are named loudly,
  so every exposure of a secret is a visible `reveal` in the source. Getting a secret's value
  is always a deliberate act, never incidental. Reached by call or UFCS: `reveal(token)` or
  `token.reveal()` (both still require the capability).
- **The pair is asymmetric on purpose.** `reveal` (string) is the short, common name because a
  secret is overwhelmingly a string (token, password, key-as-text); `revealBytes` is the
  marked exception for binary payloads. This mirrors the string API's convention of a plain
  name for the common operation and a qualified name for the variant.
- **Extraction is statically checked** against the payload kind (§3.1): `reveal` on a
  secret-bytes, or `revealBytes` on a secret-string, is a **compile error**, not a runtime
  failure. So `reveal` returns a concrete `string` with no coercion and no possibility of a
  wrong-type surprise; the payload kind guarantees the right extractor.

`revealBytes` and the `bytes` payload are **deferred** until the `bytes` type exists; `reveal`
and secret strings are the present surface.

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

- **Wraps `string`** (now) and **`bytes`** (deferred, when the `bytes` type exists). These are
  the sensitive-payload types: text and raw bytes.
- **Does not wrap `any`.** There is no real use for a secret table, secret function, secret
  command, or secret regex; admitting arbitrary payloads adds nonsense cases for no benefit.
  Because the payload set is small and its kind is carried in the type (§3.1), `reveal` /
  `revealBytes` return a concrete type rather than `any`, and the right extractor is checked
  at compile time, which is why revealing needs no coercion.
- **Sensitive compound data is modeled as secret *fields*, not a secret container.** A
  credentials object is an ordinary table whose sensitive leaf values are secret
  (`['host' => "db.example", 'password' => pw as secret]`), so the non-sensitive structure stays
  readable and secrecy is localized to the actual sensitive values.

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

- **`reveal` as a keyword:** `reveal` / `revealBytes` require the `reveal` capability
  (capabilities spec), which is what makes non-revelation a guarantee. What remains open is
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
