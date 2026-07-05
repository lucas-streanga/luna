/**
 * Luna syntax highlighting for Astro/MDX via Shiki.
 *
 * SETUP:   npm i -D shiki
 *          (shiki is a transitive dep of Astro, present at runtime; installing it
 *          directly is what gives TypeScript the `LanguageRegistration` type and
 *          resolves TS2307 on the import below. Dev-only: the import is type-only,
 *          erased at build.)
 *
 * WIRING (astro.config.mjs):
 *
 *   import { defineConfig } from 'astro/config';
 *   import mdx from '@astrojs/mdx';
 *   import { lunaGrammar } from './tooling/shiki-luna';
 *
 *   export default defineConfig({
 *     integrations: [mdx()],
 *     markdown: {
 *       shikiConfig: {
 *         langs: [lunaGrammar],      // registers ```luna fences in .md and .mdx
 *       },
 *     },
 *   });
 *
 * Then fence code blocks with ```luna. That is the whole integration: Shiki
 * accepts TextMate grammars as plain objects, and highlighting is lexical, so
 * this file is generated from the ratified lexical surface (keywords.md,
 * operators.md, associativity.md) and needs no LSP. The LSP, when it exists,
 * adds *semantic* tokens in editors through its own channel; docs never need it.
 *
 * Source of truth: keywords.md (R32-R47), operators.md (R29), strings §13.
 * Regenerate by hand when those change; the sets below cite their sections.
 */

import type { LanguageRegistration } from 'shiki';

export const lunaGrammar: LanguageRegistration = {
  name: 'luna',
  scopeName: 'source.luna',
  fileTypes: ['luna'],
  patterns: [
    { include: '#comments' },
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
        { name: 'comment.line.double-slash.luna', match: '//.*$' },
      ],
    },

    // #[jsonTag('user_name')]  — attributes spec §3
    attributes: {
      patterns: [
        {
          name: 'meta.attribute.luna',
          begin: '#\\[',
          end: '\\]',
          beginCaptures: { 0: { name: 'punctuation.definition.attribute.luna' } },
          endCaptures: { 0: { name: 'punctuation.definition.attribute.luna' } },
          patterns: [
            { name: 'entity.name.function.attribute.luna', match: '[a-zA-Z_][a-zA-Z0-9_]*' },
            { include: '#strings' },
            { include: '#numbers' },
          ],
        },
      ],
    },

    // Double quotes interpolate ($ident and ${expr}); single quotes are plain. strings §13.
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
              beginCaptures: { 0: { name: 'punctuation.section.interpolation.begin.luna' } },
              endCaptures: { 0: { name: 'punctuation.section.interpolation.end.luna' } },
              patterns: [{ include: 'source.luna' }],
            },
            {
              // "$x", "$...args" (spread §5)
              name: 'meta.interpolation.simple.luna',
              match: '\\$(\\.\\.\\.)?[a-zA-Z_][a-zA-Z0-9_]*',
              captures: { 0: { name: 'variable.other.interpolated.luna' } },
            },
          ],
        },
        {
          name: 'string.quoted.single.luna',
          begin: "'",
          end: "'",
          patterns: [{ name: 'constant.character.escape.luna', match: '\\\\.' }],
        },
      ],
    },

    // `grep -n foo` command literals, with ${} interpolation. command spec §2, §4.
    commands: {
      patterns: [
        {
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
        { name: 'constant.numeric.double.luna', match: '\\b[0-9][0-9_]*\\.[0-9][0-9_]*\\b' },
        { name: 'constant.numeric.integer.luna', match: '\\b[0-9][0-9_]*\\b' },
      ],
    },

    keywords: {
      patterns: [
        {
          // declaration keywords — keywords.md §1
          name: 'storage.type.luna',
          match: '\\b(var|let|const|fn|constraint|proto|enum|error|capability|attribute|meta|export|import|test)\\b',
        },
        {
          // control flow — keywords.md §2
          name: 'keyword.control.luna',
          match: '\\b(if|else|foreach|while|in|break|continue|return|yield|match|where|defer|try|catch|throw|by)\\b',
        },
        {
          // word operators — keywords.md §3
          name: 'keyword.operator.word.luna',
          match: '\\b(copy|spawn|await|comptime|comptype|is|as|apply|declared|use)\\b',
        },
      ],
    },

    types: {
      patterns: [
        {
          // predeclared type names — keywords.md §5 (identifiers, not reserved; styled anyway)
          name: 'support.type.luna',
          match: '\\b(int|double|bool|string|bytes|table|list|stream|promise|view|never|any|regex|command|type|byte|number|json|csv|yaml|xml|path|file|secret|panic)\\b',
        },
        {
          // @P application refinements and @@ protocol reflection — protocols §9
          name: 'support.type.refinement.luna',
          match: '@@?[a-zA-Z_][a-zA-Z0-9_]*',
        },
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
        // longest-first, per the catalogue (operators §0) and tiers (associativity §1)
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
        // name( — call or UFCS (functions §3.4); styled as a call either way
        { name: 'entity.name.function.call.luna', match: '\\b[a-zA-Z_][a-zA-Z0-9_]*(?=\\()' },
      ],
    },
  },
};

export default lunaGrammar;
