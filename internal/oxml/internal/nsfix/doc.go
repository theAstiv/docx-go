// Package nsfix is an output post-processor that corrects the namespace
// declaration ordering bugs in encoding/xml (golang/go#9519, golang/go#13400).
// It is isolated here so the workaround can be removed in one place once the
// upstream issues are resolved.
package nsfix
