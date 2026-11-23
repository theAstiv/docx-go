# Changelog

docx-go follows [semantic versioning](https://semver.org/). **Before 1.0, the public API may change between minor versions.** Requires Go ≥ 1.23.

## Unreleased

### Changed

- A table created without column widths now gets them, the way Word sizes an inserted table, so a table of empty cells no longer collapses to a sliver in Word. This applies to `Body.AddTable`, `Header`/`Footer.AddTable` and `Paragraph.InsertTableAfter`.
  - Columns that `TableOptions.ColWidth` leaves out (nil, too short, or a zero entry) share what is left of the text width evenly, on the grid and on every cell (`w:tcW`).
  - The text width depends on where the table goes:
    - **In the body:** one text column of the section holding it: the page width less the left, right and gutter margins, divided among the section's text columns (`w:cols`; the first column's width when they are unequal).
    - **In a cell:** the cell's width (its fixed `w:tcW`, else the outer grid columns it spans) less the default cell margins.
    - **In a header or footer:** the first section that uses it.
  - When the given widths leave less than half an inch per missing column, each missing column gets an even share of the whole width instead.
  - Columns with a width are unchanged.

### Added

- `Paragraph.AddSectionBreak` and `Paragraph.SectionBreak` return a `Section` that can set its own headers and footers. Before, only `Document.LastSection` could.
- `HyperlinkOptions.AllowAnyScheme` accepts link schemes besides `http:`, `https:` and `mailto:` (see Breaking). It never accepts `javascript:`, `vbscript:` or `data:`.
- `ErrInvalidPartName`: `Open` refuses a file with an entry name no part may have (absolute, with a backslash, or with an empty, `.` or `..` segment), or with two entries for one part, exactly or ignoring case.
- Templating: `Options.MaxCopies` bounds the paragraphs, tables and rows `{{#each}}` may copy in one `Execute` (default `DefaultMaxCopies`, 100,000; negative for no limit). Beyond it, `Execute` fails with `ErrOutputLimit` in both modes.

### Fixed

- `RunFont.SetColor` removes a theme colour (`w:themeColor`, `themeTint`, `themeShade`) from the run. Word shows a theme colour in place of the hex value, so a colour set on a run Word had given a theme colour did not show in Word.
- `RunFont` and `ParagraphFormat` setters called with their clearing value (`nil`, `""`) no longer add an empty `w:rPr` / `w:pPr` to a run or paragraph that has none.
- `Cell`, `Header` and `Footer.SetText` clean up like `Remove`: the other end of a bookmark or comment range they drop is removed too, and `Document.Bookmarks` stops listing dropped bookmarks.
- `Section.SetMargins` on a section without page margins writes the `header`, `footer` and `gutter` values the schema requires.
- Text extraction (`Paragraph.Text`, `Document.Text`, and the skip counts of `Replace`) follows nested fields: an index entry (`XE`, `TC`) no longer hides the rest of its paragraph, and a field inside another field's instruction no longer leaks its result.
- A table style read without `w:pPr`/`w:rPr` no longer writes `w:tblPr` before a run format set later, which the validator rejected.
- `Open`: a `.rels` file outside a `_rels` folder is an ordinary part, and `_rels/x.rels` belongs to part `x` (it was renamed on save). A malformed relationships part of a part that is not read no longer fails `Open`. Content types, relationship sources and media names compare ignoring case, so a case variant no longer adds a duplicate `Default` or overwrites an image.
- `wp:docPr` and bookmark ids stay within their ranges when a document already uses the largest one, and a huge `rId` number no longer makes new relationship ids overflow.
- Templating: a block or row range the data drops takes the other end of any bookmark or comment range it cut with it, and a removed bookmark that a cross-reference points at is a warning, as for inline ranges. A block whose branch or tag paragraph holds part of a field, or whose tag paragraph holds a drawing, is `ErrSplitAcrossBoundary` / `ErrMisplacedBlock` whatever the data.
- Templating: a header or footer emptied by a range keeps a final empty paragraph.
- Templating: only custom XML parts that a content control binds are filled. Add-in or SharePoint data holding `{{` no longer fails `Execute` or gets blanked.
- Templating: a value whose `String` method panics (a nil pointer) is reported as having no text form instead of crashing `Execute`, and a `json.Number` gets the same checks as a numeric string (`1e100000` is refused by `number` and `currency`).
- Templating: a paragraph with thousands of split tags is normalised in one pass; it was quadratic.

### Breaking

- `Paragraph.AddHyperlink` accepts only `http:`, `https:` and `mailto:` URLs by default; other schemes (`ftp:`, `file:`, `tel:`, …) need `HyperlinkOptions.AllowAnyScheme`. Windows handler schemes such as `ms-msdt:` and `search-ms:`, and `file:` links to remote shares, start programs or leak credentials when clicked. Template rich-text links come from data and always use the allow-list.
- Unit constructors (`Inches`, `Cm`, `Mm`, `Pt`, `Px`, `Percent`, `EMUInches`, `MultipleLine`, `HalfPoints`) round to the nearest unit instead of truncating (`Mm(1)` is 57, not 56) and saturate instead of overflowing.
- `Open` refuses packages with invalid or duplicate part names (`ErrInvalidPartName`).
- Templating: `{{#each}}` loops stop at 100,000 copied elements by default (`Options.MaxCopies`).
- Templating: `{{else}}` is now a keyword. A data key named `else` can no longer be written as `{{else}}`; inside a loop, `{{.else}}` still reads it.
- Templating: template errors no longer depend on the data. `Execute` now checks every branch for unbalanced or misplaced block tags, section breaks inside blocks and malformed tags in content controls, including blocks the data drops and loops over empty lists. A template with such an error in a dropped block used to succeed; it now fails in both modes. Values in branches that are not rendered are still not required. An error repeated by a loop is reported once.

### Added

- Templating: `{{else}}` in `{{#if}}`, `{{#unless}}` and `{{#each}}`. It goes alone in a paragraph, or at the start of a row's first cell in a row range, where it begins the rows of the else branch. In `{{#each}}`, the else branch shows once, with the outer data, when the list is empty, `null` or missing; without an `{{else}}`, a missing list is still a missing key. `Placeholders` reports it with kind `"else"`.
- Templating: formatters. A value tag pipes its value through them, left to right: `{{total | currency "€" 2}}`, `{{due | date "2 Jan 2006"}}`, `{{name | upper | default "—"}}`.
  - Built in: `upper`, `lower`, `default "x"` (for a missing key, `null` or `""`), `number N ["thousands" "decimal"]`, `currency "symbol" [N ["thousands" "decimal"]]` and `date "layout"` (RFC 3339 input, Go layout).
  - Arguments are quoted strings or numbers. Word's curly quotes count as quotes.
  - `number` and `currency` round half away from zero on the decimal value, so `2.675` gives `2.68`.
  - `Options.Funcs` adds formatters, or replaces built-ins other than `default` (for example, a `number` with local digit grouping). A panic in one is reported as an error.
  - New sentinels: `ErrUnknownFormatter` (fails in both modes, even in a branch the data drops) and `ErrFormat` (a value a formatter rejects: an error in Strict mode, a warning in Lenient mode). A wrong argument to a built-in is `ErrMalformedTag`.
  - `Placeholder.Formatters` lists a tag's formatter names.
- Templating: conditions inside a paragraph. `Dear {{#if vip}}valued {{/if}}customer` and `{{#if ok}}pass{{else}}fail{{/if}}` keep or remove the text between the tags, and the paragraph stays. Inline ranges nest, and work inside paragraph and row ranges, loops and hyperlinks.
  - The removed text may hold tabs, line breaks, inline pictures, bookmarks, and whole hyperlinks, fields, content controls and tracked changes.
  - A range that would remove part of a field, one end of a bookmark or comment, or a footnote, endnote or comment reference is `ErrSplitAcrossBoundary`, whichever branch the data selects. Every such range is reported in one run.
  - Removing a bookmark that a cross-reference (`REF`, `PAGEREF`, `NOTEREF`) or internal link points at is a warning: Word would show "Error! Reference source not found." there.
  - `{{#each}}` inside a paragraph is `ErrMisplacedBlock`. So is a range that opens in one paragraph and closes in another, as before.
  - A block tag at a table row's edge that closes inside its own paragraph is now an inline range, not the start of a row range. The exception is a range without `{{else}}` that fills a one-cell row on its own, which stays a row range: `{{#each items}}- {{.}}{{/each}}` in a one-column row still repeats the row.
  - Block tags in content controls and text boxes are still `ErrUnsupportedLocation`.
- Templating: image values. `{{image logo}}` places a picture inline, in the tag's run, from a `template.Image{Data, Width, Height, AltText, Title}` value (also `*template.Image`) anywhere in the data.
  - PNG, JPEG and GIF, validated and sized as `Run.AddImage` does: both sizes zero gives the pixel size at 96 DPI, and one zero follows from the aspect ratio.
  - `AltText` is required. A missing or invalid image, a non-image value in an `{{image}}` tag, or an image in a plain value tag is `ErrUnsupportedValue`. A missing key follows Strict or Lenient mode, and in Lenient mode nothing is placed.
  - Works in the body, tables, headers and footers (the relationship goes on the part that holds the tag) and in loops: every copy gets its own drawing id, and identical bytes are stored once. Image tags in content controls and text boxes are `ErrUnsupportedLocation`.
  - `Execute` stays atomic. Adding an image changes the package itself (media part, relationship, content type), so `Execute` records the package before the first image and restores it if execution fails.
- Templating: rich-text values. A value tag whose value is a `template.Rich{Runs: []template.RichRun{…}}` writes formatted runs in place of the tag. Each `RichRun` has `Text` (with `\n` and `\t`), tri-state `Bold`, `Italic` and `Underline`, a `Color` (`RRGGBB`) and a `Link`.
  - Each run starts from the formatting of the run holding the tag, and its own settings apply on top. The text before and after the tag keeps the tag run's formatting.
  - A `Link` becomes an external hyperlink with the document's Hyperlink style. The URL is checked as for `Paragraph.AddHyperlink`, and the relationship goes on the part holding the tag, headers and footers included.
  - A linked run drops the tag run's direct colour and underline, unless the `RichRun` sets them, so it looks like a link. The Hyperlink style replaces any character style the tag had, with a warning.
  - A link where the tag already sits inside a hyperlink, or in the result of a `HYPERLINK` field, is `ErrUnsupportedLocation`; rich runs without links work there.
  - An empty rich value inserts nothing and leaves the tag's run whole. New runs do not copy the tag run's tracked formatting change, whose id must stay unique.
  - A bad colour or an unsafe or relative URL is `ErrUnsupportedValue`. Formatters other than `default` reject a rich value (`ErrFormat`).
  - It is a typed value, not markup, so data cannot inject document content. In content controls, text boxes and document properties the plain text is written, with a warning.
  - A failed `Execute` also undoes links: the relationship and the Hyperlink style it added are removed.

## v0.4.0: editing existing documents

Open a document, then remove and insert content, find and replace text, read its plain text and set its properties. Templates can be filled without writing Go, using the new `docxfill` command.

Every document this release generates passes the OOXML schema validator and opens without a "Repaired document" prompt in LibreOffice (checked in CI). Word and Google Docs were checked by hand. [`examples/edit`](examples/edit) edits the report that [`examples/report`](examples/report) builds.

### Added

- `Document.Text()`, `Body.Text()`, `Header.Text()` and `Footer.Text()` return a story's plain text. Every paragraph ends with `\n`. In a table, cells are separated by `\t` and each row ends with `\n`, and a cell's paragraphs are joined with `\n`. Headers and footers are not part of `Document.Text()`. Text in text boxes is not included yet.
- `Document.CoreProperties()` reads and writes the core document properties (`docProps/core.xml`): `Title`, `Subject`, `Author`, `Keywords`, `Description`, `Category`, `LastModifiedBy`, `Revision`, `Created` and `Modified`.
  - Setting a property on a document without the part creates it. An empty string or a zero time removes the property.
  - A document whose properties are not changed keeps `core.xml` byte-for-byte.
  - Properties this API does not name are kept.
  - `Modified` and `LastModifiedBy` are not updated automatically on save.
- Paragraph tab stops, borders and shading on `ParagraphFormat`, so they work for paragraphs and paragraph styles alike:
  - Tab stops: `TabStops`, `SetTabStops`, `AddTabStop`, `RemoveTabStop` and `ClearTabStop`, with `TabAlign*` alignments and `TabLeader*` fill characters. Stops are written sorted by position. `ClearTabStop` cancels a stop inherited from the style.
  - Borders: `SetBorders(ParagraphBorderOptions{Top, Left, Bottom, Right, Between, Bar})`, `Borders` and `ClearBorders`.
  - Shading: `SetShading(fill)` and `Shading`.
- Structural editing of existing documents:
  - Removal: `Paragraph.Remove()`, `Table.Remove()`, `Row.Remove()` and `Run.Remove()`. They return `ErrNotInDocument` (already removed), `ErrSectionBreak` (the paragraph carries a section break), `ErrUnbalancedField` (the content holds only part of a field, such as the start of a table of contents) or `ErrLastRow`.
  - A removal also removes the other end of any bookmark or comment range it cuts.
  - A cell, header or footer always keeps a final paragraph.
  - Removing the row where a vertical merge starts passes the start to the row below.
  - Insertion: `Paragraph.InsertParagraphBefore/After(text)`, `Table.InsertParagraphBefore/After(text)`, `Paragraph.InsertTableAfter(rows, cols, opts)` and `Table.InsertRow(i, opts)`.
  - `InsertRow` copies the neighbouring row's cell structure, widths, borders, shading and paragraph formatting, and extends a vertical merge it is inserted into.
- Find and replace: `Document.Replace(old, new, ReplaceOptions)` and `Paragraph.Replace`.
  - Text that Word split across runs (spell-check marks, formatting or revision changes) is found. The replacement takes the formatting of the run holding the match's first character.
  - `\n` and `\t` in the replacement become a line break and a tab.
  - Options: `IgnoreCase`, `SkipHeaders`, `SkipFooters` and `Limit`.
  - The result reports `Count`, and in `Skipped` the matches that could not be replaced: those split by a field, bookmark or hyperlink edge, those inside content controls or tracked insertions, and those inside a table of contents. Field results are replaced; field instructions are not searched.
- `cmd/docxfill`: fill a template from JSON on the command line.
  - `docxfill [-lenient] -o out.docx template.docx data.json`, where `data.json` may be `-` for stdin and `-o -` writes to stdout.
  - `docxfill -placeholders template.docx` lists the template's tags.
  - Warnings go to stderr. Exit status is 2 for template or data errors (each one listed), and 1 for usage or file errors.
  - The output is written only on success, never partially.

### Fixes

- A table created without a style (`AddTable(…, TableOptions{})`) had no `w:tblPr`, which the schema requires. It now always has one.
- `TableOptions.ColWidth` was written only on the table grid. Word lays out an autofit table from the cell widths, so a table whose cells were still empty collapsed to a sliver (LibreOffice was not affected). The width is now also written on every cell of the column (`w:tcW`).

### Behaviour changes

- `Paragraph.Text()` (and `Hyperlink.Text()`) now include text inside content the library keeps unparsed: tracked insertions (`w:ins`), smart tags, inline content controls, simple fields and custom XML. Tracked deletions (`w:del`, `w:moveFrom`) are still excluded. Before, all of this text was silently dropped.
- `Cell.Text()` now includes nested tables (laid out as in `Body.Text()`) and content controls in the cell.
- Templating: whether a paragraph is a block tag (`{{#each}}`, `{{/if}}`, …) is now decided on its whole visible text, including text in tracked insertions and content controls. A block tag typed with track changes on now works. A block-tag paragraph that also holds other inserted text used to be removed along with that text; it is now reported as a misplaced block tag.

## v0.3.0: report building and templating

Everything needed to generate a professional report: from scratch with the API, or by filling a Word template from JSON data.

Every document this release generates, and every template it fills, passes the OOXML schema validator and opens without a "Repaired document" prompt in Word, LibreOffice and Google Docs. LibreOffice is checked in CI on every change; the Word and Google Docs checks were done by hand.

### Templating: new package `docx/template`

Write placeholders in an ordinary Word document, then fill it:

```go
warnings, err := template.Execute(doc, data, template.Options{Mode: template.Strict})
```

- **Tags:** `{{client.name}}` values with dotted paths, `{{#each}}` loops, `{{#if}}` and `{{#unless}}` blocks. Inside a loop, `{{.field}}` is a field of the current item and `{{.}}` is the item itself. Loops and blocks nest.
- **Where block tags go:** alone in their own paragraph, so the paragraphs and tables between the two tags repeat or disappear; or at the edges of table rows, so the rows repeat.
- **Where tags work:** body, tables, headers and footers, plus content controls and text boxes, such as Word's built-in cover pages.
- **Word's run splits:** a tag that Word stored in several pieces, because of spell-check marks, formatting changes or revision ids, is rejoined using the formatting of the piece that holds `{{`.
- **Data-bound content controls** (a cover page's Title or Date) show values stored in the document properties or custom XML. Those are filled too, so Word shows the filled value when it opens the file.
- **Tables of contents** repeat heading text, tags included. They are left for Word to rebuild: call `doc.SetUpdateFieldsOnOpen(true)`. `Execute` warns if a TOC holds tags and you haven't.
- **Modes and errors:** `Strict` (the default) returns one `*template.Error` that lists every missing key and malformed tag, with its location, and leaves the document unchanged. `Lenient` writes missing values as empty text and returns warnings.
- **Checking a template:** `template.Placeholders(doc)` lists a template's tags, so data can be checked against it before rendering.
- **Repeated content** gets fresh drawing ids and keeps bookmarks on the first copy only, so Word sees no duplicates.
- **Tested** against templates made in Word itself.

Not supported yet: `{{else}}`, helpers, conditions inside a paragraph, rich-text and image values, and tags in footnotes, endnotes and comments (`Execute` warns about those).

### Report building

| Feature | API |
|---|---|
| Inline images | `Run.AddImage(r, ImageOptions)` (PNG, JPEG, GIF), `Run.Images()`, `Image.Size/SetSize/AltText/Bytes`; identical images are stored once |
| Hyperlinks | `Paragraph.AddHyperlink(url, opts)`, `AddInternalLink(bookmark, opts)`, `Hyperlinks()`, `Hyperlink.AddRun/URL/Anchor/Text` |
| Bookmarks | `Paragraph.AddBookmark(name)`, `Document.Bookmark(name)`, `Document.Bookmarks()` |
| Styles | `Document.Styles()`, `Styles.ByID/All/AddParagraphStyle/AddCharacterStyle/AddTableStyle/EnsureBuiltin`, `Style.Format()/Font()/SetBasedOn/SetNext`, document defaults |
| Table formatting | `Table/Cell.SetBorders(BorderOptions)`, `SetWidth(TableWidth)`, `Table.SetCellMargins/SetLayout`, `Cell.SetShading`, `Row.SetHeader/SetCantSplit` |
| Fields | `Paragraph.AddField/AddPageNumber/AddPageCount/Fields()`, `Document.AddTOC(opts)`, `Document.SetUpdateFieldsOnOpen(bool)`; `Fields()` also finds fields that span paragraphs, such as a Word-made TOC |
| Cell, header and footer text | `Cell/Header/Footer.SetText(s)` replaces the content with one paragraph and returns it |

Examples: [`examples/report`](examples/report) builds a report with every feature; [`examples/template_report`](examples/template_report) fills a template from JSON.

### Behaviour changes

Check these when upgrading from v0.2.0:

- **`Cell`, `Header` and `Footer.AddParagraph` now append**, like `Body.AddParagraph`. Before, they inserted before the last paragraph. A new cell, header or footer holds one empty paragraph: fill it with `SetText`, or `AddParagraph` adds a second one.
- **`Header` and `Footer.AddTable` append** the table, followed by an empty paragraph, because Word wants a header or footer to end with one.
- **Properties are written in schema order.** Paragraph, run and section properties are sorted into the order the schema requires when saved. Input that was out of order is normalised; valid input is unchanged.
- **Left and right indents** are written as `w:left` / `w:right`, which every Word version accepts. `w:start` / `w:end` are still read.
- **`Styles.EnsureBuiltin`** returns the document's existing equivalent style when there is one, such as `Standard` in a localized template instead of a second `Normal`. Apply the returned `Style.ID()`, not the constant.
- **`AddBookmark`, `AddHyperlink`, `AddInternalLink` and `AddTOC` return an `error`** for invalid names, URLs or options.

### Fixes

- **Revision ids kept:** paragraphs and runs keep their `w:rsid*` and `w14:paraId` attributes on save. Before, an unchanged save dropped them.
- **Headers and footers:** header and footer references are written before the page size and margins, as the schema requires. Every v0.2.0 document with a header or footer was affected.
- **Table rows:** `w:tblPrEx` is written before `w:trPr`.
- **Unique ids:** drawing and bookmark ids stay unique across the whole document, including inside content controls, tracked changes and text boxes. Duplicates are a cause of Word's "unreadable content" warning.
- **Empty defaults:** an empty `w:rPrDefault` / `w:pPrDefault` is written back as it was.
- **Elements in no namespace:** an unknown element in no namespace inside a namespaced one is written with `xmlns=""`. Before, it fell into its parent's namespace on save, and on the next open it was read back as a different element.
- **Faster many-image reports:** adding many images or bookmarks no longer rescans the whole document for each one.

## v0.2.0: tables, lists, sections, headers and footers

First tagged release. Licensed under Apache-2.0.

- Paragraphs and runs with paragraph and run formatting.
- Tables with merged cells.
- Numbered and bulleted lists with restart.
- Multi-section documents with default, first-page and even-page headers and footers.
- Document settings (`EvenAndOddHeaders`, `DefaultTabStop`).
- Unmodelled content is kept on round-trip: same names, attributes, child order and text. Namespace prefixes may be re-canonicalised.
