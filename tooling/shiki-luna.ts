/**
 * Luna syntax highlighting for Astro/MDX via Shiki.
 *
 * SETUP:   npm i -D shiki        (type-only dep; Astro ships the runtime)
 * WIRING:  markdown.shikiConfig.langs: [lunaGrammar] in astro.config.mjs,
 *          then ```luna fences highlight. Astro's <Code> also accepts this
 *          object directly as `lang`.
 *
 * SOURCE OF TRUTH: specs/build/lexer.md (the lexer specification, G-rulings
 * revision) + keywords.md. This file is DERIVED from the lexer's token
 * inventory (§2 comments, §3 keywords, §4 literals, §5 operators, §6
 * interpolation, §7 identifiers); span patterns use the spec's unrolled-loop
 * form verbatim. Now-ruled facts this grammar embodies (were assumptions):
 * `from` is reserved (G1); block comments exist and do NOT nest, so the
 * begin/end pair is the complete rule (G3); command literals have NO escape
 * sequences, backslash is an ordinary byte and literal backticks arrive via
 * interpolation (G5, command §2.2); identifiers are formal ASCII over UTF-8
 * sources (G6, lexical-structure §1-2); `match!` is one token (G7). The
 * vscode-luna tmLanguage is extracted from THIS file (CHANGES R59): edit
 * here, then regenerate. Highlighting-grade caveats: the F2 regex/division
 * ambiguity is approximated with a lookbehind heuristic (oniguruma has
 * lookbehind; the real lexer uses prev-token state), and interpolation
 * inside regex bodies is not sub-highlighted.
 */

// No import from 'shiki': this repo has no node_modules, and the type was only
// documentation. Shiki accepts grammars STRUCTURALLY, so a minimal local type
// keeps this file self-contained and editor-quiet everywhere. On the website
// (where shiki is installed), an optional one-liner restores the real check:
//   import type { LanguageRegistration } from 'shiki';
//   const _check: LanguageRegistration = lunaGrammar;

type TmRule = {
  name?: string;
  match?: string;
  begin?: string;
  end?: string;
  captures?: Record<string, { name: string }>;
  beginCaptures?: Record<string, { name: string }>;
  endCaptures?: Record<string, { name: string }>;
  patterns?: TmRule[];
  include?: string;
};

export type LunaGrammar = {
  name: string;
  scopeName: string;
  fileTypes: string[];
  patterns: TmRule[];
  repository: Record<string, { patterns: TmRule[] }>;
};

export const lunaGrammar: LunaGrammar = {
  name: 'luna',
  scopeName: 'source.luna',
  fileTypes: ['luna'],
  patterns: [
    { include: '#comments' },
    { include: '#bytes' },
    { include: '#regex' },
    { include: '#attributes' },
    { include: '#strings' },
    { include: '#commands' },
    { include: '#numbers' },
    { include: '#keywords' },
    { include: '#types' },
    { include: '#constants' },
    { include: '#operators' },
    { include: '#functions' },
  ],
  repository: {
    comments: {
      patterns: [
        { name: 'comment.line.double-slash.luna', match: '//[^\\n]*' },
        { name: 'comment.block.luna', begin: '/\\*', end: '\\*/' },
      ],
    },

    bytes: {
      patterns: [
        { name: 'string.quoted.double.bytes.luna', match: 'b"[^"\\\\]*(?:\\\\.[^"\\\\]*)*"' },
        { name: 'string.quoted.single.bytes.luna', match: "b'[^'\\\\]*(?:\\\\.[^'\\\\]*)*'" },
      ],
    },

    regex: {
      patterns: [
        {
          name: 'string.regexp.luna',
          match: '(?<![\\w)\\]])(/[^/\\\\*][^/\\\\]*(?:\\\\.[^/\\\\]*)*/)([imsxb]*)',
          captures: {
            '1': { name: 'string.regexp.body.luna' },
            '2': { name: 'keyword.other.regexp-flags.luna' },
          },
        },
      ],
    },

    attributes: {
      patterns: [
        {
          name: 'meta.attribute.luna',
          begin: '#\\[',
          end: '\\]',
          beginCaptures: { '0': { name: 'punctuation.definition.attribute.luna' } },
          endCaptures: { '0': { name: 'punctuation.definition.attribute.luna' } },
          patterns: [
            { name: 'entity.name.function.attribute.luna', match: '[A-Za-z_][A-Za-z0-9_]*' },
            { include: '#strings' },
            { include: '#numbers' },
          ],
        },
      ],
    },

    strings: {
      patterns: [
        {
          name: 'string.quoted.double.luna',
          begin: '"',
          end: '"',
          patterns: [
            { name: 'constant.character.escape.luna', match: '\\\\.' },
            {
              name: 'meta.interpolation.luna',
              begin: '\\$\\{',
              end: '\\}',
              beginCaptures: { '0': { name: 'punctuation.section.interpolation.begin.luna' } },
              endCaptures: { '0': { name: 'punctuation.section.interpolation.end.luna' } },
              patterns: [{ include: 'source.luna' }],
            },
            {
              name: 'meta.interpolation.simple.luna',
              match: '\\$(\\.\\.\\.)?[A-Za-z_][A-Za-z0-9_]*',
              captures: { '0': { name: 'variable.other.interpolated.luna' } },
            },
          ],
        },
        {
          name: 'string.quoted.single.luna',
          match: "'[^'\\\\]*(?:\\\\.[^'\\\\]*)*'",
        },
      ],
    },

    commands: {
      patterns: [
        {
          // No escape sub-pattern, by ruling (G5, command §2.2): backslash is
          // an ordinary byte inside command literals.
          name: 'string.interpolated.command.luna',
          begin: '`',
          end: '`',
          patterns: [
            {
              name: 'meta.interpolation.luna',
              begin: '\\$\\{',
              end: '\\}',
              patterns: [{ include: 'source.luna' }],
            },
          ],
        },
      ],
    },

    numbers: {
      patterns: [
        { name: 'constant.numeric.hex.luna', match: '\\b0x[0-9a-fA-F](?:_?[0-9a-fA-F])*\\b' },
        { name: 'constant.numeric.binary.luna', match: '\\b0b[01](?:_?[01])*\\b' },
        { name: 'constant.numeric.double.luna', match: '\\b[0-9](?:_?[0-9])*\\.[0-9](?:_?[0-9])*(?:[eE][+-]?[0-9]+)?\\b' },
        { name: 'constant.numeric.double.luna', match: '\\b[0-9](?:_?[0-9])*[eE][+-]?[0-9]+\\b' },
        { name: 'constant.numeric.integer.luna', match: '\\b[0-9](?:_?[0-9])*\\b' },
      ],
    },

    keywords: {
      patterns: [
        { name: 'keyword.control.luna', match: '\\bmatch!' },
        {
          name: 'storage.type.luna',
          match: '\\b(var|let|const|fn|constraint|proto|enum|error|capability|attribute|meta|export|import|from|test)\\b',
        },
        {
          name: 'keyword.control.luna',
          match: '\\b(if|else|foreach|while|in|break|continue|return|yield|match|where|defer|try|catch|throw|by)\\b',
        },
        {
          name: 'keyword.operator.word.luna',
          match: '\\b(copy|spawn|await|comptime|comptype|is|as|apply|declared|use)\\b',
        },
      ],
    },

    types: {
      patterns: [
        {
          name: 'support.type.luna',
          match: '\\b(int|double|bool|string|bytes|table|list|stream|promise|view|never|any|regex|command|type|byte|number|json|csv|yaml|xml|path|file|secret|panic)!?',
        },
        { name: 'support.type.refinement.luna', match: '@@?[A-Za-z_][A-Za-z0-9_]*' },
      ],
    },

    constants: {
      patterns: [
        { name: 'constant.language.luna', match: '\\b(true|false|null|undefined|self)\\b' },
        { name: 'variable.language.wildcard.luna', match: '\\b_\\b' },
      ],
    },

    operators: {
      patterns: [
        { name: 'keyword.operator.assignment.compound.luna', match: '(\\?\\?\\?=|\\?\\?=|\\+=|-=|\\*=|/=|%=)' },
        { name: 'keyword.operator.coalescing.luna', match: '(\\?\\?\\?|\\?\\?|\\?\\.)' },
        { name: 'keyword.operator.pipeline.luna', match: '\\|>' },
        { name: 'keyword.operator.arrow.luna', match: '(=>|->)' },
        { name: 'keyword.operator.range.luna', match: '(\\.\\.<|\\.\\.\\.|\\.\\.)' },
        { name: 'keyword.operator.comparison.luna', match: '(==|!=|<=|>=|<|>)' },
        { name: 'keyword.operator.logical.luna', match: '(&&|\\|\\||!)' },
        { name: 'keyword.operator.arithmetic.luna', match: '(\\+|-|\\*|/|%)' },
        { name: 'keyword.operator.other.luna', match: '(&|\\||\\?|=|:)' },
      ],
    },

    functions: {
      patterns: [
        { name: 'entity.name.function.call.luna', match: '\\b[A-Za-z_][A-Za-z0-9_]*(?=\\()' },
      ],
    },
  },
};

export default lunaGrammar;
