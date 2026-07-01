# The Luna Programming Language

Luna is a general-purpose hybrid scripting and ahead-of-time compiled programming language. Luna is data-focused, with an emphasis on a compositional approach to programming. It is heavily inspired by Raku, Lua, PHP, and Go.

At the time of writing, no Luna implementation exists. The intention is to eventually provide a `luna` binary, which will act as the full suite of tooling for Luna, including a runner, compiler, formatter, LSP, and even static library for embedding.

The plan is for Luna to use a goland backend. Meaning, the compiler will compile Luna code to Go, which will then be compiled to native binaries for the platform. The reason Go is preliminarily chosen as the compiler backend is:

- The semantics of Go are close enough to Luna that there is not a large dissonance.
- Go is a well-performing language.
- Utilizing Go grants us a high-performance garbage collector for free.
- Utilizing Go grants us multiple target platforms for free.
- Utilizing Go grants us some compiler backend optimizations for free.
- The Go compiler is very fast, enabling the creation of Luna as a scripting language.

# Proposed compiler usage
`$ luna myprogam.luna`

The above will run the provided program. Imports are filesystem based, so this one file may import many luna modules. The artifacts of the build will be cached at `$HOME/.lunalang/`, so on subsequent runs there is no start-up cost. In addition, incremental rebuilds will be supported, for faster start-up times.

`$ luna -c myprogram.luna` or `$ luna --compile myprogram.luna`

The above will compile the provided program, and provide a completely self-contained platform binary. There will be no dynamic dependence on libc or the Go runtime; both will be statically linked and self-contained. This method will use the same binary caching logic as above, along with incremental rebuilds.

Additional run and compile flags: `-d`, `--debug`. Runs the compiler in debug mode, emitting debug symbols. The default build method is release mode.

`$ luna -f myprogram.luna` or `$ luna --format myprogram.luna`

The above will run the built-in Luna formatter on the provided program, including recursively through all its imported modules.
