# luna
The Luna Programming Language

At the time of writing, no implementation exists. Only a spec and some barebones tooling exists.

# Syntax highlighting (zed)
Requires `podman`, `podman compose`, and `git`

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
