package highlight

// StyleSheet is a complete default theme for the classes Render emits.
//
// Colours go through custom properties so a site can retheme without restating the rules:
// every declaration below reads a `--luna-*` variable, and overriding one variable on
// `.luna` changes that colour everywhere it is used.
//
// Both themes live in one declaration each, via light-dark(). The obvious alternative, a
// prefers-color-scheme block redefining every variable, states the palette twice,
// and a palette stated twice is a palette that drifts. It also only answers the *system*
// preference, so a site with its own light/dark toggle would leave code blocks stranded in
// the wrong theme. Here the toggle has a hook:
//
//	:root[data-theme="dark"] { color-scheme: dark; }
//
// Classes rather than the inline colours Shiki bakes into each span: one stylesheet for a
// whole site, both themes with no second render, and a palette change that diffs as the
// palette rather than as every page.
const StyleSheet = `.luna {
  color-scheme: light dark;

  --luna-bg:     light-dark(#fbfbfa, #0d1117);
  --luna-fg:     light-dark(#24292f, #c9d1d9);
  --luna-com:    light-dark(#6a737d, #8b949e);
  --luna-kw:     light-dark(#cf222e, #ff7b72);
  --luna-decl:   light-dark(#8250df, #d2a8ff);
  --luna-const:  light-dark(#0550ae, #79c0ff);
  --luna-var:    light-dark(#953800, #ffa657);
  --luna-type:   light-dark(#1a7f37, #7ee787);
  --luna-num:    light-dark(#0550ae, #79c0ff);
  --luna-str:    light-dark(#0a3069, #a5d6ff);
  --luna-esc:    light-dark(#cf222e, #ff7b72);
  --luna-regex:  light-dark(#116329, #7ee787);
  --luna-cmd:    light-dark(#0a3069, #a5d6ff);
  --luna-interp: light-dark(#cf222e, #ff7b72);
  --luna-op:     light-dark(#24292f, #c9d1d9);
  --luna-punc:   light-dark(#57606a, #8b949e);
  --luna-attr:   light-dark(#6e7781, #8b949e);
  --luna-err:    light-dark(#cf222e, #f85149);

  background: var(--luna-bg);
  color: var(--luna-fg);
  padding: 1rem;
  overflow-x: auto;
  border-radius: 6px;
  line-height: 1.5;
  tab-size: 4;
}

.luna .tok-com    { color: var(--luna-com); font-style: italic; }
.luna .tok-kw     { color: var(--luna-kw); }
.luna .tok-decl   { color: var(--luna-decl); }
.luna .tok-const  { color: var(--luna-const); }
.luna .tok-var    { color: var(--luna-var); }
.luna .tok-type   { color: var(--luna-type); }
.luna .tok-num    { color: var(--luna-num); }
.luna .tok-str    { color: var(--luna-str); }
.luna .tok-esc    { color: var(--luna-esc); font-weight: 600; }
.luna .tok-regex  { color: var(--luna-regex); }
.luna .tok-cmd    { color: var(--luna-cmd); }
.luna .tok-interp { color: var(--luna-interp); font-weight: 600; }
.luna .tok-op     { color: var(--luna-op); }
.luna .tok-punc   { color: var(--luna-punc); }
.luna .tok-attr   { color: var(--luna-attr); }

/* INVALID covers bytes no production claims (R242). A docs build should never ship one --
   cmd/highlight -strict is what stops it -- so this is the visible backstop, not a style. */
.luna .tok-err {
  color: var(--luna-err);
  text-decoration: underline wavy var(--luna-err);
}
`
