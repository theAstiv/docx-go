# Fixture Sources

Files under `word/` were created manually in Microsoft Word for this project.

The `word/template_*.docx` templating fixtures (plan task P3b.8.1) were made by
Microsoft Word 16 driven through COM automation, so Word itself wrote every
part. Each file went through three Word sessions (type, toggle formatting
mid-tag, edit a letter inside a tag and restore it), with background spell-check
on and personal information removed. The run splits, `w:proofErr` marks and
revision ids are Word's own.

Files under `python-docx/` are from the [python-docx](https://github.com/python-openxml/python-docx) project,
licensed under the **MIT License**. Copyright © python-docx contributors.

Files under `poi/` are from the [Apache POI](https://github.com/apache/poi) project,
licensed under the **Apache License 2.0**. Copyright © The Apache Software Foundation.

Run `bash scripts/fetch-fixtures.sh` to re-download or update these files.
