# Security policy

## Supported versions

docx-go is pre-1.0. Security fixes go into the latest minor release only.

| Version | Supported |
|---|---|
| 0.3.x | Yes |
| < 0.3 | No |

## Reporting a vulnerability

**Do not open a public issue for a security problem.**

Report it privately through GitHub:
[**Report a vulnerability**](https://github.com/theAstiv/docx-go/security/advisories/new).

Please include:

- the docx-go version and Go version;
- a description of the problem and its impact;
- a minimal reproducer: code, and a `.docx` file if the problem is triggered
  by input. Strip any confidential content from the file first.

You should get a first response within 7 days. Once the problem is confirmed,
a fix is released and a GitHub security advisory is published, crediting you
unless you prefer otherwise.

## What counts

docx-go opens `.docx` files, which are ZIP archives of XML, and these often
come from untrusted sources. Problems in reading them are in scope, for
example:

- a crafted file that makes `docx.Open` panic, hang, or use unbounded memory
  or CPU (ZIP bombs, deeply nested XML, very large parts);
- a ZIP entry name that makes the library read or write outside the archive;
- a document that docx-go writes, or a template it fills, in a way that lets
  input data inject markup (fields, hyperlinks, `javascript:` URLs) the caller
  did not ask for.

Bugs that only produce a wrong or invalid document, with no security impact,
are ordinary bugs: please open an issue for them.
