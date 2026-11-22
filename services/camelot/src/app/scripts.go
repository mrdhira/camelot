package app

import "slices"

// Component names a shadcn-templ component whose script a page loads.
type Component string

// Components with a script.
const (
	ComponentDialog   Component = "dialog"
	ComponentProgress Component = "progress"
	ComponentSwitch   Component = "switch"
	ComponentTooltip  Component = "tooltip"
)

// bundleOrder is the module's bundle order of the scripts in js/shadcn/:
// the shared blocks first, then the components.
var bundleOrder = []string{
	"floating_focus_manager.js",
	"lifecycle.js",
	"mark_others.js",
	"portal.js",
	"scroll_lock.js",
	"tabbable.js",
	"use_anchor_positioning.js",
	"use_click.js",
	"use_dismiss.js",
	"use_focus.js",
	"use_hover.js",
	"use_transition_status.js",
	"dialog.js",
	"floating_ui_core.js",
	"floating_ui_dom.js",
	"progress.js",
	"switch.js",
	"tooltip.js",
}

// scriptsOf returns a component's script with the module's scripts it depends on.
func scriptsOf(c Component) []string {
	switch c {
	case ComponentDialog:
		return []string{"lifecycle.js", "portal.js", "use_dismiss.js", "scroll_lock.js", "use_transition_status.js", "tabbable.js", "mark_others.js", "floating_focus_manager.js", "use_click.js", "dialog.js"}
	case ComponentProgress:
		return []string{"lifecycle.js", "progress.js"}
	case ComponentSwitch:
		return []string{"lifecycle.js", "switch.js"}
	case ComponentTooltip:
		return []string{"lifecycle.js", "portal.js", "use_dismiss.js", "use_transition_status.js", "use_anchor_positioning.js", "tabbable.js", "use_hover.js", "use_focus.js", "floating_ui_core.js", "floating_ui_dom.js", "tooltip.js"}
	}
	return nil
}

// Scripts returns the asset paths a page loads: the components' scripts once
// each in bundle order, then the page's own (js/<name>.js).
func Scripts(components []Component, page ...string) []string {
	need := map[string]bool{}
	for _, c := range components {
		for _, s := range scriptsOf(c) {
			need[s] = true
		}
	}
	var out []string
	for _, s := range bundleOrder {
		if need[s] {
			out = append(out, "js/shadcn/"+s)
		}
	}
	return slices.Concat(out, page)
}

// isLayoutScript reports whether the layout already loads s.
func isLayoutScript(s string) bool {
	return slices.Contains(Scripts(layoutComponents, "js/live.js", "js/palette.js", "js/panel.js"), s)
}
