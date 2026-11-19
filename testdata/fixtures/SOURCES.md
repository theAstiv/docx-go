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

Files under `python-docx/` are from the [python-docx](https://github.com/python-openxml/python-docx) project,
licensed under the **MIT License**. Copyright © python-docx contributors.

Files under `poi/` are from the [Apache POI](https://github.com/apache/poi) project,
licensed under the **Apache License 2.0**. Copyright © The Apache Software Foundation.

Run `bash scripts/fetch-fixtures.sh` to re-download or update these files.
