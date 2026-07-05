# Luna for VS Code (local install)

Editor-side highlighting for `.luna` files and ```` ```luna ```` fences in markdown.
The grammar is **generated from** `tooling/shiki-luna.ts` (one source of truth), so
the site's Shiki highlighting and the editor's stay in lockstep by regeneration, not
discipline.

## Install (no marketplace, no build)

    cp -r tooling/vscode-luna ~/.vscode/extensions/luna-lang-0.0.1

(Windows: `%USERPROFILE%\.vscode\extensions\`.) Then run `Developer: Reload
Window`. `.luna` files highlight immediately; markdown fences tagged `luna` highlight
too, because VS Code builds its fence patterns from registered language ids.

## Regenerating after grammar changes

`syntaxes/luna.tmLanguage.json` is derived from the Shiki grammar; re-run the
extraction (CHANGES.md R59) rather than editing it by hand.
