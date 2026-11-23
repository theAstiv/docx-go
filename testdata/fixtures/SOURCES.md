# Fixture Sources

Files under `word/` were created manually in Microsoft Word for this project.

The `word/template_*.docx` templating fixtures (plan task P3b.8.1) were made by
Microsoft Word 16 driven through COM automation, so Word itself wrote every
part. Each file went through three Word sessions (type, toggle formatting
mid-tag, edit a letter inside a tag and restore it), with background spell-check
on and personal information removed. The run splits, `w:proofErr` marks and
revision ids are Word's own.

`word/template_formatters.docx` (P3c.8) was made the same way. COM typing does
not run AutoFormat As You Type, so after typing, Word's own Find/Replace of `"`
and `'` with smart quotes on turned the formatter arguments into curly quotes
(`“€”`, `‘n/a’`). The second session bolded `totl | curr` across the key and
the formatter name; the third replaced and restored a letter inside `“not
given”`, `January` and `lower`, so Word split those into separate runs.

`word/template_inline_if.docx` (P3c.10) was made the same way. The first
session typed inline `{{#if}}`/`{{#unless}}` ranges with and without
`{{else}}`, a nested range, a tab and a line break inside branches, a range
in a hyperlink's display text and ranges inside a `{{#each}}` row. The second
bolded `vip}}val` and `ndings{{else}}no` and italicised a letter run inside the
kept text of a row range; the third replaced and restored a letter inside
`show_totals`, `{{/unless}}`, `asessor` and `{{/if}}`.

`word/template_images.docx` (P3c.6) was made the same way. The first session
typed `{{image}}` tags in the body, in the default header (followed by text),
in the second cell of a `{{#each team}}` row (`{{image .photo}}`) and in both
branches of inline `{{#if}}`/`{{#unless}}` conditions. The second bolded
`image sig` inside `{{image signature}}` and italicised the `{{im` that starts
the header tag; the third replaced and restored a letter inside `logo`,
`.photo`, `badge` and the header's `client_logo`. Its data holds images, which
JSON cannot express, so `TestWordTemplates_Images` builds the data in Go and
writes `filled-template_images.docx` for `generate-docs.sh`.

`word/template_rich.docx` (P3c.9) was made the same way. The first session
typed rich-value tags: one per paragraph, two in one run (`{{owner}}`,
`{{ref}}`), one inside the display text of a hyperlink Word made
(`{{link_text}}`), one in the last cell of a `{{#each mitigations}}` row
(`{{.state}}`) and one in the default header (`{{doc_title}}`). The second
made `{{status}}` italic, `{{sum` bold, `{{.state}}` navy and `{{doc_` bold,
so the rich runs have formatting to start from; the third replaced and
restored a letter inside `status`, `owner`, `link_text`, `.control` and
`doc_title`. Rich values cannot come from JSON, so `TestWordTemplates_Rich`
builds the data in Go and writes `filled-template_rich.docx` for
`generate-docs.sh`.

Files under `python-docx/` are from the [python-docx](https://github.com/python-openxml/python-docx) project,
licensed under the **MIT License**. Copyright © 2013 Steve Canny. The licence text is in `NOTICE`.

Files under `poi/` are from the [Apache POI](https://github.com/apache/poi) project,
licensed under the **Apache License 2.0**. Copyright © The Apache Software Foundation.
The attribution from Apache POI's `NOTICE` is in `NOTICE`.

Run `bash scripts/fetch-fixtures.sh` to re-download these files. It fetches them
from fixed upstream commits and checks each against `SHA256SUMS`; to add or
update a fixture, bump the commit in the script and add its checksum.
