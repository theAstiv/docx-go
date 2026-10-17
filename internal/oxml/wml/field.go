package wml

// Field character types for w:fldChar/@w:fldCharType.
const (
	FldCharBegin    = "begin"
	FldCharSeparate = "separate"
	FldCharEnd      = "end"
)

// NewFieldRuns builds the five runs of a complex field in the layout Word
// writes: begin, instruction, separate, cached result, end.
//
//	<w:r><w:fldChar w:fldCharType="begin"/></w:r>
//	<w:r><w:instrText xml:space="preserve"> PAGE </w:instrText></w:r>
//	<w:r><w:fldChar w:fldCharType="separate"/></w:r>
//	<w:r><w:t>1</w:t></w:r>
//	<w:r><w:fldChar w:fldCharType="end"/></w:r>
//
// The cached result is stored exactly as given; field results are never
// computed here. An empty cachedResult yields a result run with no text.
func NewFieldRuns(instr, cachedResult string) []*CT_R {
	fldChar := func(typ string) *CT_R {
		fc := new(CT_FldChar)
		fc.FldCharType = typ
		r := new(CT_R)
		r.Children = append(r.Children, fc)
		return r
	}

	it := new(CT_InstrText)
	it.Space = "preserve"
	it.Content = " " + instr + " "
	instrRun := new(CT_R)
	instrRun.Children = append(instrRun.Children, it)

	resultRun := new(CT_R)
	if cachedResult != "" {
		t := new(CT_Text)
		t.Space = "preserve"
		t.Content = cachedResult
		resultRun.Children = append(resultRun.Children, t)
	}

	return []*CT_R{
		fldChar(FldCharBegin),
		instrRun,
		fldChar(FldCharSeparate),
		resultRun,
		fldChar(FldCharEnd),
	}
}
