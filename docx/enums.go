package docx

// AlignmentValue is the set of paragraph alignment values (w:jc/@w:val).
type AlignmentValue string

const (
	AlignLeft     AlignmentValue = "left"
	AlignCenter   AlignmentValue = "center"
	AlignRight    AlignmentValue = "right"
	AlignJustify  AlignmentValue = "both"
	AlignStart    AlignmentValue = "start"
	AlignEnd      AlignmentValue = "end"
	AlignDistribute AlignmentValue = "distribute"
	AlignThaiDistribute AlignmentValue = "thaiDistribute"
)

// LineSpacingRule is the rule used for w:spacing/@w:lineRule.
type LineSpacingRule string

const (
	LineSpacingAuto    LineSpacingRule = "auto"
	LineSpacingExact   LineSpacingRule = "exact"
	LineSpacingAtLeast LineSpacingRule = "atLeast"
)

// VertAlignValue is the set of vertical text alignment values (w:vertAlign/@w:val).
type VertAlignValue string

const (
	VertAlignBaseline    VertAlignValue = "baseline"
	VertAlignSuperscript VertAlignValue = "superscript"
	VertAlignSubscript   VertAlignValue = "subscript"
)

// UnderlineStyle is the set of underline style values (w:u/@w:val).
type UnderlineStyle string

const (
	UnderlineNone             UnderlineStyle = "none"
	UnderlineSingle           UnderlineStyle = "single"
	UnderlineWords            UnderlineStyle = "words"
	UnderlineDouble           UnderlineStyle = "double"
	UnderlineThick            UnderlineStyle = "thick"
	UnderlineDotted           UnderlineStyle = "dotted"
	UnderlineDottedHeavy      UnderlineStyle = "dottedHeavy"
	UnderlineDash             UnderlineStyle = "dash"
	UnderlineDashHeavy        UnderlineStyle = "dashedHeavy"
	UnderlineDashLong         UnderlineStyle = "dashLong"
	UnderlineDashLongHeavy    UnderlineStyle = "dashLongHeavy"
	UnderlineDotDash          UnderlineStyle = "dotDash"
	UnderlineDotDashHeavy     UnderlineStyle = "dashDotHeavy"
	UnderlineDotDotDash       UnderlineStyle = "dotDotDash"
	UnderlineDotDotDashHeavy  UnderlineStyle = "dashDotDotHeavy"
	UnderlineWave             UnderlineStyle = "wave"
	UnderlineWavyHeavy        UnderlineStyle = "wavyHeavy"
	UnderlineWavyDouble       UnderlineStyle = "wavyDouble"
)
