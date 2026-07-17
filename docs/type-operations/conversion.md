# Conversion

Turning a value of one type into a value of another is done by **functions**, never by `as`.
This document is the canonical home for that distinction and for the conversion story across the
language: how `as` differs from a conversion, why rendering out is total while parsing in is
fallible, how `toString` is made open to user types through a protocol without overloading, and
which conversions are extensible versus closed.

---

## 1. `as` asserts a type; conversion transforms a value

The dividing line, stated once here and relied on everywhere:

- **`as` is checked narrowing.** It asserts that a value **already is** some type (a union member,
  a subtype) and re-types it, runtime-checked, raising a `typeError` (panic) on mismatch. It
  **never changes the value** (`as` spec). `x as int` means "x already is an int, treat it as
  one," not "make an int out of x."
- **A conversion is a function.** It **produces a different value**, of a different type: `true`
  to `1`, `5` to `"5"`, `"5"` to `5`. Because the value changes, this is a function, never `as`.

So the rule is: **if the value is unchanged and you are only re-typing it, that is `as`; if a new
value of a different type is produced, that is a conversion function.** Every `toX` / `parseX` /
`fromX` named below is a function for exactly this reason, and no conversion is ever spelled with
`as`. (`byte`-style constraint narrowing is `as`, because a `byte` *is* an `int`; converting an
`int` to a `string` is a function, because a string is not an int.)

---

## 2. Rendering out is total; parsing in is fallible

Conversions split by direction, and the two directions have opposite failure profiles:

- **Rendering out (a value to a `string`) is total.** Every value has *a* string form, so
  `toString` cannot fail (§3). There is always a sensible or at worst default rendering, so
  producing a string never errors.
- **Parsing in (a `string` to a value) is fallible.** Not every string denotes a valid value of
  the target type (`"abc"` is not an `int`, `"maybe"` is not a `bool`), so `parseX` / `fromString`
  returns a fallible `T!` (a declarable error on a string that does not denote a `T`, errors spec).

This asymmetry is inherent: rendering has the full value in hand and only formats it (always
possible), while parsing must *recover* a value from text that may not encode one (sometimes
impossible). So **out is total, in is fallible**, and the `!` sits on the parsing side only.

```
toString(value): string          // total: always succeeds
parseInt(s):     int!             // fallible: not every string is an int
parseBool(s):    bool!            // fallible
fromString(s):   T!               // fallible in general
```

---

## 3. `toString` is one function, opened to user types by a protocol

Luna has **no function overloading** (language overview), so there is a **single** `toString`,
not one per type:

```
const toString = fn (value: any): string => { ... };     // total; one function for all types
```

A single function still renders every type appropriately, and stays **open** to user types,
because it **dispatches through a protocol member** rather than to overloads. A type makes
itself renderable by applying the well-known **`stringify` protocol**, whose one member is a
**required fn-typed member**: each application supplies its own renderer at apply, as an
initializer (protocols §4.2), immutable thereafter —

```
stringify = proto {
  const get toString: fn (any): string;   // required: bound per application, at apply
};

var user = ['name' => n] apply stringify(toString: fn (u: any): string => { ... });
```

— and `toString` (the free function) dispatches through that member when the protocol is
applied, falling back to a built-in rendering otherwise:

```
const toString = fn (value: any): string => match {
  value is @stringify => value->stringify.toString(value),   // the application's own renderer
  _                   => builtinRender(value),               // default per built-in type
};
```

Every step of that dispatch is a form the protocol model already permits, which is the point:

- **`value is @stringify`** is a membership test on the applied set (the `@@` axis,
  protocols §8), the sanctioned application test (is spec).
- **`value->stringify.toString`** is the **qualified form** of `->` (protocols §3.1): the
  member `toString` of protocol `stringify`, read as a stored `fn` value and called with
  the value explicitly — a fn-typed member call (protocols §3.4), not a protocol-function
  call, which is what lets every application carry a *different* renderer. Qualification
  pins the protocol deliberately: bare `value->toString(value)` would resolve against
  whatever protocols happen to be in scope at the call site, and a dispatch entry point
  should not depend on its own import list.

- **`toString`** is the **free function callers use**. It is total (§3.1).
- **`stringify`** is the **protocol types have applied**. The two names stay distinct on purpose:
  callers call `toString`, types apply `stringify`; the entry point and the extension point
  never blur. How an application supplies its rendering is the ordinary initializer
  mechanism — a required member, bound at apply (protocols §4.2) — nothing
  conversion-specific.

So **custom rendering is a protocol member, not overloading and not lookup**: a user type
has `stringify` applied with its own renderer bound, and `toString` reaches it by name.
A required fn-typed member is how any interface-shaped contract is expressed under the
member model (protocols §2); `toString` extensibility needs no new machinery. (One
consequence to know: the renderer is a `get` fn-typed per-table member, so it sits in the
serialization surface and meets the fn-serialization rule — `toJson` on a `@stringify`
value raises `typeError` unless `skipFunctions` is set; json §2.1.)

### 3.1 `toString` is total, including for user types

Because `toString` is total, it always yields a string:

- For a **built-in** type, the built-in rendering always exists (a number, a bool, a string
  itself, an enum variant, a table, each has a default form).
- For an **application** of `stringify`, the bound member supplies the rendering, and its
  totality is **contract, not courtesy**: the member's declared type is `fn (any): string`,
  no `!`, and an apply initializer is checked against the member's declared type
  (protocols §4.2), so an errorable implementation cannot enter the protocol. Display
  paths (logging, interpolation, `.`
  concatenation, strings §11) therefore never fail through the extension point; only ambient
  panics remain possible, as everywhere (errors §7).

This totality is what lets **string interpolation** (string-builder spec) lower to `toString`
calls safely: interpolating any value, built-in or user, always produces text, so `"${anyValue}"`
never crashes.

### 3.2 Rendering respects value semantics, including redaction

`toString` is a display path, so values that **redact on display** stay redacted through it. A
`secret` renders as its redaction placeholder, never its underlying payload (secret spec), because
`secret`'s rendering is defined to redact. So opening `toString` to protocols does not open a
hole: a type's `stringify` controls its *own* rendering, and built-in redacting types render
safely by their built-in rule. `toString` never bypasses a type's display contract.

---

## 4. Which conversions are open, and which are closed

Not every conversion is user-extensible, and the split is principled:

- **`toString` is open** (extensible via the `stringify` protocol), because **display is
  universal**: any value might be logged, interpolated, or shown, so every type, including user
  types, needs a rendering. The open protocol dispatch is what makes that universal.
- **The numeric and parsing family is closed** (`toInt`, `toDouble`, `parseInt`, `parseBool`,
  `fromString` for built-ins): these are **built-in-to-built-in** conversions with fixed
  meaning, and there is no general sense in which an arbitrary user type "converts to `int`." A
  user type that *does* have a natural numeric projection provides a **named function of its own**
  (`account.toBalance()`, `color.toRgb()`), not a hook into a universal `toInt`. So custom numeric
  conversion is an ordinary named function on the type, not an extension point.

The reason for the asymmetry: **display is a universal need (open); type-specific conversions are
specific needs (named functions).** So the language provides one open coercion (`toString`), a
closed family of built-in conversions, and leaves type-specific conversions to ordinary named
functions, no overloading anywhere.

---

## 5. Conversions between built-in types

The built-in conversion functions (each total or fallible per §2), specified in the relevant type
specs and summarized here:

- **To string (total):** `toString(value): string` for any value (§3).
- **From string (fallible):** `parseInt(s): int!`, `parseBool(s): bool!`, and the like, one per
  parseable built-in (string spec and each type's spec).
- **Numeric (between number types):** `toDouble(n: int): double` (total, lossy above 2^53),
  `toInt(d: double): int!` (fallible: nan, infinities, out-of-range have no int; int spec,
  double spec).
- **Bool (total out):** `toInt(b: bool): int` (`true` to 1), `toString(b)` (`true` to `"true"`);
  there is deliberately no `int`-to-`bool` conversion (write the comparison; bool spec §3).

Each of these is a **function** (§1), reachable by UFCS (`b.toInt()` is `toInt(b)`), and none is
spelled with `as`.

---

## 6. Open questions

- **A separate debug rendering.** Whether a second rendering (a `toDebugString` / `inspect` that
  shows structure, quotes strings, and does not redact for a trusted debug context) exists
  alongside `toString`, and how it interacts with `secret` (it must still not reveal secrets by
  default). Deferred.
- **`fromString` as a general open coercion.** Whether parsing *into* user types is ever opened by
  a protocol (a required `parse` member — the `stringify` pattern in reverse) the way rendering out
  is, or whether parsing stays a closed built-in family with user types providing their own named
  parsers. Leaning closed, pending need.
- **Locale and formatting options.** Whether `toString` ever takes formatting options (number
  grouping, precision) or whether formatted rendering is a separate, explicit function family, so
  that bare `toString` stays a single canonical rendering. Deferred to a formatting design.
