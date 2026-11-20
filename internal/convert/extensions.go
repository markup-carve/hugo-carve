package convert

import carve "github.com/markup-carve/carve-go/v2"

// The Carve 0.2 standard extensions that make up the bundle: every name in
// carve.StandardExtensions except carve.inlineFootnotesToParentheses, which
// rewrites footnotes and was never part of the 0.1 bundle. The 0.1 navigation
// extensions (heading permalinks, toc, external links, autolink) have no 0.2
// counterpart in carve-rs yet (markup-carve/carve-rs#4476).
var bundleExtensionKeys = func() []string {
	var keys []string
	for _, name := range carve.StandardExtensions {
		if name != "carve.inlineFootnotesToParentheses" {
			keys = append(keys, name)
		}
	}
	return keys
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
