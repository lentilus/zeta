module.exports = grammar({
  name: 'mdlink',

  // allow whitespace/newlines as extras so blank lines don't produce ERRORs
  extras: $ => [/\s+/],

  rules: {
    // allow an optional metadata/front-matter block at the top,
    // followed by the normal inline content
    source_file: $ => seq(
      optional($.metadata),
      repeat($._inline)
    ),

    // YAML-like front matter between '---' and '---'
    // e.g.
    // ---
    // key1 : val1
    // Key2 : val2
    // ---
    metadata: $ => seq(
      '---',
      repeat($.metadata_pair),
      '---'
    ),

    // a single metadata key/value pair, captures key & value fields
    // spaces around the colon are allowed because whitespace is an extra
    metadata_pair: $ => seq(
      field('key', $.metadata_key),
      ':',
      field('value', $.metadata_value)
    ),

    // key is any run of characters up to a colon or newline
    metadata_key: $ => token(/[^\n:]+/),

    // value is the rest of the line (may be empty)
    metadata_value: $ => token(/[^\n]*/),

    source_line: $ => /.*/,

    _inline: $ => choice(
      $.link,
      /[^\[\]\(\)\n]+/       // plain text
    ),

    // link node with labeled child nodes
    link: $ => seq(
      '[', field('text', $.label), ']',   // capture label as a `label` node
      '(', field('destination', $.destination), ')' // capture destination as `destination`
    ),

    // make label/destination explicit node types (token rules)
    label: $ => /[^\]\n]+/,
    destination: $ => /[^\)\n]+/,
  }
});
