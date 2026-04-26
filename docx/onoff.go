package docx

// On returns a *bool pointing to true, meaning "explicitly enabled".
// Assign to a *bool proxy field to turn a CT_OnOff property on.
func On() *bool { v := true; return &v }

// Off returns a *bool pointing to false, meaning "explicitly disabled".
// Assign to a *bool proxy field to turn a CT_OnOff property off.
func Off() *bool { v := false; return &v }

// Inherit returns nil, meaning "inherit from the applied style".
// This is the zero value for any *bool proxy field; the function exists only
// for readability at call sites.
func Inherit() *bool { return nil }
