package convert

// The extension registry keys that make up the full bundle.
//
// carve-go up to v0.1.4 sent the whole bundle for any non-empty slice, so this
// package asked for it with the single name "all". v0.1.5 selects by registry
// key instead, and "all" is not a key: it is refused, so every page rendered
// with -extensions or -static failed outright.
//
// A recorded list goes stale the moment the registry gains a key.
// TestBundleExtensionsIsTheWholeRegistry holds it to the engine's own registry
// by asking the engine, so a new extension fails this package rather than
// being silently left out of the bundle.
var bundleExtensionKeys = []string{
	"autolink",
	"citations",
	"code-callouts",
	"code-group",
	"color-swatch",
	"details",
	"external-links",
	"fenced-render",
	"fenced-render-abc",
	"fenced-render-chart",
	"fenced-render-d2",
	"fenced-render-graphviz",
	"fenced-render-plantuml",
	"fenced-render-vega-lite",
	"fenced-render-wavedrom",
	"glossary",
	"heading-level-shift",
	"heading-numbers",
	"heading-permalinks",
	"heading-reference",
	"img-fence",
	"index",
	"list-table",
	"math-block",
	"smart-quotes",
	"semantic-span",
	"spoiler",
	"tabs",
	"tab-normalize",
	"table-of-contents",
	"toc",
	"wikilinks",
}

// bundleExtensions returns a fresh copy, so a caller holding the slice in
// carve.Options cannot reorder or truncate the package's own list.
func bundleExtensions() []string {
	out := make([]string, len(bundleExtensionKeys))
	copy(out, bundleExtensionKeys)
	return out
}
