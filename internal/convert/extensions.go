package convert

import carve "github.com/markup-carve/carve-go"

// The Carve 0.2 standard extensions that make up the bundle: every name in
// carve.StandardExtensions except carve.inlineFootnotesToParentheses, which
// rewrites footnotes and was never part of the 0.1 bundle, plus the dot and
// puml fence words of the graphviz and plantuml presets.
var bundleExtensionKeys = func() []string {
	var keys []string
	for _, name := range carve.StandardExtensions {
		if name != "carve.inlineFootnotesToParentheses" {
			keys = append(keys, name)
		}
	}
	// A named preset claims only its own fence word; 0.1 graphviz also
	// claimed dot, and plantuml also puml.
	return append(keys, "carve.fencedRender.dot", "carve.fencedRender.puml")
}()

// bundleRenderers selects the standard renderer of every bundled extension
// that has one.
func bundleRenderers() map[string]string {
	out := make(map[string]string, len(bundleExtensionKeys))
	for _, name := range bundleExtensionKeys {
		if name != "carve.cite" {
			out[name] = "standard"
		}
	}
	return out
}

// bundleExtensions returns a fresh copy, so a caller holding the slice in
// carve.Options cannot reorder or truncate the package's own list.
func bundleExtensions() []string {
	out := make([]string, len(bundleExtensionKeys))
	copy(out, bundleExtensionKeys)
	return out
}
