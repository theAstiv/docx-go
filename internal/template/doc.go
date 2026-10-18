// Package template is the templating engine behind docx/template.
//
// It fills {{placeholders}}, {{#if}} / {{#unless}} blocks and {{#each}}
// loops in WordprocessingML block trees ([]wml.BlockElement) from
// JSON-shaped data. It works on the element layer only: it never imports
// internal/parts, internal/opc or docx (enforced by depguard). The public
// package docx/template hands it the document's stories (body, headers and
// footers) through docx/internal/bridge.
//
// Architecture: docx/template → internal/template → internal/oxml
package template
