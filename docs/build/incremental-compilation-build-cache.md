# Incremental compilation and the build cache

The compiler caches **per-module compiled binary artifacts** in a home-directory cache, so an
unchanged module is not recompiled and only **linking** remains. This document specifies that
cache: the key that decides when an artifact is reused, the cheap-but-correct change detection, the
version namespacing, and the eviction and sharing model. It is a companion to the compiler spec
(which defines the phase pipeline the cache plugs into); section references of the form "compiler
spec §N" point there.

The design turns on the cache key, which must be **correct** (never serve a stale artifact) and
**cheap** (avoid `open()` on the hot path).

---

## 1. The cache key is transitive over the module interface

A module's artifact depends on its own source **and** on the **public interface of everything it
imports**: if an imported module changes a function's result type, a dependent's artifact is stale
even though the dependent's own file is untouched. So the key is:

```
key(M) = own_key(M) + { interface_hash(I) : I in imports(M) }
```

where `interface_hash(I)` fingerprints `I`'s **public interface**, the surface a dependent compiles
against. Keying only on a module's own file would silently miscompile when a dependency's interface
changes (wrong output that looks correct), so the transitive key is mandatory, not an optimization.
Because the module graph is a DAG (modules spec §2), interface hashes propagate cleanly along
topological order.

### 1.1 The primitive is interface extraction, not a checksum

The compiler extracts each module's **public interface** as a structured artifact, and
`interface_hash` is a deterministic fingerprint of that artifact. This is deliberately **not** a
whole-file checksum (comments stripped or not), and the difference is the whole point of the
scheme.

A checksum answers "did the file change?" The interface hash answers the narrower question "did the
public *interface* change?", and only the second enables the **recompile-versus-relink** split that
makes compilation actually incremental:

- If an imported module's **interface hash is unchanged** (only bodies changed), its dependents'
  keys are unchanged, so they **do not recompile**; they **relink** against the module's new
  artifact. Their emitted code is still valid because the interface they compiled against did not
  move.
- If the **interface hash changed**, dependents **recompile** (re-analyze and re-emit against the
  new interface).

A checksum cannot make this distinction: any body-only edit changes the checksum, so every
dependent recompiles. Editing one function body in a widely-imported module would then recompile
its entire dependency cone, the "change one file, rebuild the world" cascade, even though every
dependent's generated code is identical. The interface hash cuts the cascade at every module whose
interface did not change, so a body-only edit recompiles just that module and relinks the rest.
Body-only edits are the most common kind of change, so this is where the savings concentrate. A
checksum is *correct* (it over-invalidates, which is safe) but pessimal; the interface hash is what
makes incremental builds incremental.

### 1.2 What the public interface includes

The interface is what a dependent could compile differently against, reached transitively:

- **Every exported binding and its full type**, functions (all of the signature: parameters,
  result, errorability, and comptime-eligibility, since all four are type identity, functions spec
  §3), consts and their types, enums, constraints, protocols, capabilities.
- **The full structural content of each exported type**, not just its name: an enum's complete
  variant set and payload types (dependents' `match` exhaustiveness and destructuring depend on
  them, match spec §9), a constraint's **predicate** (dependents' `as` checks and check-elision
  depend on it, constraints spec §10), a protocol's full meta-function surface and element
  contract. Two definitions that share a name but differ in content must hash differently.
- **Transitively-reachable types, even private ones exposed through a public signature.** If an
  exported `fn f(): Internal` returns a non-exported `Internal`, then `Internal`'s structure is
  observable through `f` (a dependent can `match` and destructure the result), so `Internal`'s
  structure is in the interface even though it is not itself exported. Missing this silently
  miscompiles dependents when a "private" type that leaks through a public API changes.
- **The bodies of comptime-eligible exported functions.** Comptime (functions spec §5) can execute
  an imported function at compile time, so a dependent's compile-time result depends on that
  function's **body**, not just its signature. For a comptime-eligible exported function the body
  is therefore part of the observable interface and is included. (An ordinary exported function's
  body is not, §1.3.)

### 1.3 What the interface excludes (the payoff)

Everything a dependent cannot observe is excluded, which is exactly what a whole-file checksum
cannot exclude and what makes the interface hash worth having:

- **Ordinary (non-comptime) function bodies.** Changing an exported function's implementation
  without changing its type does not change its interface, so dependents relink rather than
  recompile.
- **Private definitions not reachable from any exported type**, a helper used only internally is
  not in the interface; changing it recompiles the module itself (via `own_key`, §2) but does not
  cascade to dependents.
- **Comments, formatting, and private names**, unobservable, excluded.

So the interface is a true **interface fingerprint**: it changes only when a dependent could
compile differently, and implementation churn (the common case) does not cascade.

### 1.4 One primitive, several consumers

Interface extraction is a single primitive with more than one consumer, so it is not cost
attributable to the cache alone:

- **The incremental cache** uses `interface_hash` for change detection and the recompile-versus-
  relink split (above).
- **The in-compiler tooling** (formatter, linter, LSP, all to be specified later) must compute a
  module's public interface regardless, navigation, hover types, find-references, and "what does
  this module export" all need it. So the tooling builds interface extraction anyway, which makes
  the cache's hash a near-free fingerprint of an artifact that already exists, and guarantees the
  cache and the tooling agree on what a module's interface is (they read the same extraction).
- **Future binary distribution** (compiler spec §10) can reuse the same fingerprint as a
  compatibility check: an opaque compiled artifact carries its interface hash, and a consumer
  verifies it matches what they linked against.

Because the tooling requires interface extraction independently, using a checksum for the cache
would be a false economy: it would forgo the recompile-versus-relink savings *and* maintain a
second, weaker notion of "what changed" beside the interface the tooling already computes.

---

## 2. `stat()`-first, hash-on-change

`own_key(M)` uses the filesystem cheaply on the common path and hashes only when something changed:

- **Fast path (no `open()`):** if `(mtime, size)` **matches** the cached entry, the module is
  unchanged, reuse its artifact. This is a `stat()` only, no file read, for the overwhelming
  common case of "nothing changed."
- **Change path (hash to confirm):** if `(mtime, size)` **differs** from the cached entry, the
  module is *possibly* changed, so **read and content-hash** it. This costs an `open()`, but only
  when the module was going to be recompiled anyway, so the hash is nearly free. Hashing on this
  path closes the two silent-miscompile holes that pure `stat()` leaves:
  - a **size-preserving edit** (`<` to `>`, `+` to `-`) that `size` cannot see, and
  - a **preserved or coarse `mtime`** (`cp -p`, `rsync --times`, `tar`, `git checkout`, coarse-
    granularity filesystems) that makes a changed file look unchanged or an unchanged file look
    changed.

  If the content hash matches the cached entry despite the differing `(mtime, size)`, the artifact
  is reused (and the entry's `(mtime, size)` refreshed), so a `touch` or a no-op checkout does not
  force a recompile. If the hash differs, the module is recompiled.

So `stat()` decides "unchanged" on the hot path with no file read, and a content hash decides on the
cold path (where a read is already implied). This pays neither the silent-miscompile risk of
pure-`stat()` nor the always-`open()` cost of always-hashing.

One residual blind spot is accepted and documented, not engineered against: a size-preserving edit
whose `mtime` is restored to the *exact* value the cache recorded (which needs a timestamp-pinning
tool, not normal editing or a `git` checkout, both of which move `mtime` forward) matches
`(mtime, size)` on the fast path and is not detected. This is effectively unreachable through
ordinary editor-and-git workflows; when a workflow does normalize or restore timestamps in place,
the remedy is a manual full rebuild (`--clean`), which the cache cannot infer on its own.

---

## 3. The cache is namespaced by compiler version

Cache entries are stored under a **compiler-version-stamped** namespace, because a new compiler may
emit different Go or different artifacts for identical source. Namespacing by version prevents an
upgraded compiler from serving artifacts built by the old one. Comptime results are deterministic
(compiler spec §6, §8), so they cache cleanly under the same keying, an unchanged comptime-eligible
module reuses its computed constants.

---

## 4. Eviction and sharing

**The cache is private and per-user, not shared across users or projects.** Cross-project artifact
sharing is deliberately not done: an artifact is a compiled binary specialized to *its* build
context (comptime results, constraint-elision decisions, const-table specialization), so the same
module source built in a different context may need a different artifact; a shared cache is also a
code-injection trust boundary; and the cross-project hit rate is low. (Path relativity is *not* the
reason, the cache key is content-and-interface derived, §1, so it is already path-independent.)
Cross-project reuse, if ever wanted, belongs at the package layer (a package manager caching
packages), not at the compiler's module-artifact layer.

### 4.1 Two locks: builds share, eviction excludes

Liveness (an artifact must not be deleted while a build needs it) is protected by a **build lock**,
not by a timestamp heuristic. Reading a recent `mtime` as "probably in use" is too frail for a
delete decision, so liveness is a fact, not a guess:

- **The build lock is a shared (reader) lock.** Many concurrent builds hold it at once; it does not
  serialize builds. While any build holds it, the evictor must not delete artifacts those builds
  are using.
- **The eviction lock is exclusive (writer).** The evictor takes it to evict. So builds are
  readers and the evictor is the writer of a readers-writer lock: builds run in parallel, and
  eviction excludes (and is excluded by) active builds.

This separates concerns cleanly: `mtime` is used **only** as the last-used / age signal for LRU and
the age threshold (below), never as a liveness signal. The build lock alone decides what is in use.

### 4.2 Age-based eviction, with a size-cap backstop

Each artifact's **`mtime` is its last-used time**, bumped on every cache hit (an `utimes` call, no
file read), and read via the `stat()` the cache already performs (§2). Eviction is:

- **By age (default):** an artifact unused for more than **30 days** is evictable. Thirty days is
  chosen so a program run even occasionally, a script invoked every week or two, still finds its
  cache warm; a shorter window (a day, a week) would cold-cache occasional and script-style users,
  who are exactly the ones a cross-run cache should serve. Anything untouched for a month is
  genuinely stale (abandoned project, deleted branch, replaced dependency).
- **By size (backstop):** if the cache still exceeds a size cap after age eviction, evict
  least-recently-used entries (oldest `mtime`) until under the cap, so high-volume users cannot
  grow the cache without bound within the 30-day window.

Both the age threshold and the size cap are configurable; the defaults are 30 days and a sensible
size limit.

### 4.3 The background evictor: throttled, crash-safe, parallel

Eviction runs in a **detached background worker** so it never blocks a build, and its scheduling is
governed by the eviction lock:

- **Throttled to at most once per day, across all programs.** A build spawns an evictor only if the
  last eviction ran more than **1 day** ago (read from the eviction lock's timestamp via `stat()`).
  So the overwhelming majority of builds spawn nothing. This 1-day **throttle** (how often eviction
  runs, machine-wide) is a distinct number from the 30-day **artifact age** (how long a specific
  program's artifacts survive unused): the sweep runs daily; artifacts live for a month.
- **At most one evictor at a time.** The exclusive eviction lock is the mutex: a second would-be
  evictor that cannot take the lock simply does not run.
- **Crash-safe via steal-on-stale.** The eviction lock records the evictor's **PID and start
  time**. A would-be evictor that finds a lock whose PID is dead, or whose start time is older than
  any sane eviction run, **steals** it (deletes and re-acquires). So a crashed or killed evictor
  never disables eviction permanently, which a naive create-on-start / delete-on-finish lock would.
- **Parallel deletion within the lock holder.** Once it holds the exclusive lock, the evictor
  deletes provably-stale entries (old `mtime`, not held by any build) concurrently. Holding the
  lock plus the age threshold makes each deletion safe.

The single unifying signal is `mtime`: bumped on every use, it serves as last-used time (LRU),
age (the 30-day threshold), and nothing else, liveness is the build lock's job, so no delete
decision ever rests on a frail timestamp guess.

---

## 5. Open questions

- **Canonical interface serialization (§1).** The precise, deterministic serialization of a
  module's extracted public interface that the hash fingerprints, in particular the canonical form
  and ordering (so semantically-equal interfaces serialize identically), and the exact inclusion
  boundary already fixed in §1.2 and §1.3 (full exported types, transitively-reachable private
  types exposed through public signatures, and comptime-eligible bodies; excluding ordinary bodies,
  unexposed-private definitions, comments, and formatting). This is shared with the tooling that
  extracts the same interface.
- **Eviction tuning and cache-directory layout.** The exact default size cap, the on-disk layout
  of the version-namespaced cache directory, and whether the age threshold and size cap adapt to
  available disk, pending implementation experience.
