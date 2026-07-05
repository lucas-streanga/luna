/**
 * tree-sitter-luna: a HIGHLIGHTING-GRADE grammar, deliberately lexical.
 *
 * This is not the Luna parser (that is the compiler's job, per the spec's
 * grammar pins); it tokenizes accurately and imposes almost no structure, so
 * it never mis-parses valid code and never breaks highlighting on partial
 * code while typing. Keyword classification happens in highlights.scm via
 * #match? predicates, generated from keywords.md.
 */
module.exports = grammar({
  name: 'luna',

  extras: $ => [/\s/, $.comment],

  rules: {
    source_file: $ => repeat($._token),

    _token: $ => choice(
      $.attribute,
      $.string,
      $.command,
      $.number,
      $.identifier,
      $.punctuation,
    ),

    comment: _ => token(seq('//', /[^\n]*/)),

    attribute: _ => token(seq('#[', /[^\]]*/, ']')),

    string: _ => token(choice(
      seq('"', repeat(choice(/[^"\\]/, seq('\\', /./))), '"'),
      seq("'", repeat(choice(/[^'\\]/, seq('\\', /./))), "'"),
    )),

    command: _ => token(seq('`', repeat(choice(/[^`\\]/, seq('\\', /./))), '`')),

    number: _ => token(/[0-9][0-9_]*(\.[0-9][0-9_]*)?/),

    identifier: _ => token(/[a-zA-Z_][a-zA-Z0-9_]*!?/),

    punctuation: _ => token(choice(
      '???=', '??=', '+=', '-=', '*=', '/=', '%=',
      '???', '??', '?.', '|>', '=>', '->', '..<', '...', '..',
      '==', '!=', '<=', '>=', '&&', '||', '@@',
      /[-+*\/%=<>!?&|@:;,.(){}\[\]$]/,
    )),
  },
});
