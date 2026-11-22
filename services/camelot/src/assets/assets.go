// Package assets embeds the static files the UI serves from its own origin
// and serves them under content-hashed names: the built stylesheet, the
// font and the page scripts (js/); shadcn-templ's component scripts the
// pages load are served as js/shadcn/ from the pinned module's own
// embedded files (componentScripts).
package assets

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/axadrn/shadcn-templ/v2/components"
)

// Prefix is the URL path Serve is mounted on.
const Prefix = "/assets/"

// CacheControl is the Cache-Control of every served asset: a hashed name
// never changes content.
const CacheControl = "public, max-age=31536000, immutable"

//go:embed css/app.css fonts js/*.js
var embedded embed.FS

// componentScripts maps each shadcn-templ component script the pages load, js/shadcn/<file>, to its file under components/ in
// the module pinned in go.mod; no other file of the module is served.
var componentScripts = map[string]string{
	"js/shadcn/floating_focus_manager.js": "baseui/floating_focus_manager.js",
	"js/shadcn/lifecycle.js":              "baseui/lifecycle.js",
	"js/shadcn/mark_others.js":            "baseui/mark_others.js",
	"js/shadcn/portal.js":                 "baseui/portal.js",
	"js/shadcn/scroll_lock.js":            "baseui/scroll_lock.js",
	"js/shadcn/tabbable.js":               "baseui/tabbable.js",
	"js/shadcn/use_anchor_positioning.js": "baseui/use_anchor_positioning.js",
	"js/shadcn/use_click.js":              "baseui/use_click.js",
	"js/shadcn/use_dismiss.js":            "baseui/use_dismiss.js",
	"js/shadcn/use_focus.js":              "baseui/use_focus.js",
	"js/shadcn/use_hover.js":              "baseui/use_hover.js",
	"js/shadcn/use_transition_status.js":  "baseui/use_transition_status.js",
	"js/shadcn/dialog.js":                 "dialog/dialog.js",
	"js/shadcn/floating_ui_core.js":       "floatingui/floating_ui_core.js",
	"js/shadcn/floating_ui_dom.js":        "floatingui/floating_ui_dom.js",
	"js/shadcn/progress.js":               "progress/progress.js",
	"js/shadcn/tooltip.js":                "tooltip/tooltip.js",
}

// source is where Serve reads a served asset: name in fsys.
type source struct {
	fsys fs.FS
	name string
}

// hashed maps an asset name to its hashed name; served maps a hashed name
// back to its source. Both are built once, at startup; no content is kept
// on the heap.
var hashed, served = hashAll()

// hashAll names every embedded asset and every component script after the
// first 8 bytes of its SHA-256, in hex, inserted before the first dot of
// its base name (css/app.css → css/app-<hex>.css). A name that already
// carries that hex (fonts/geist-<hex>.woff2) is kept.
func hashAll() (map[string]string, map[string]source) {
	hashed, served := map[string]string{}, map[string]source{}
	add := func(name string, src source) error {
		content, err := fs.ReadFile(src.fsys, src.name)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(content)
		hash := hex.EncodeToString(sum[:8])
		dir, base := path.Split(name)
		hashedName := name
		if !strings.Contains(base, hash) {
			i := strings.IndexByte(base, '.')
			if i < 0 {
				i = len(base)
			}
			hashedName = dir + base[:i] + "-" + hash + base[i:]
		}
		hashed[name], served[hashedName] = hashedName, src
		return nil
	}
	err := fs.WalkDir(embedded, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		return add(name, source{fsys: embedded, name: name})
	})
	if err != nil {
		panic("assets: hash embedded files: " + err.Error())
	}
	for name, file := range componentScripts {
		if err := add(name, source{fsys: components.SourceFiles, name: file}); err != nil {
			panic("assets: hash component scripts: " + err.Error())
		}
	}
	return hashed, served
}

// Path returns the URL path of the asset name, relative to this directory
// ("css/app.css"): Prefix and its hashed name. A name that is not
// embedded keeps its name, so Serve answers it 404.
func Path(name string) string {
	if h, ok := hashed[name]; ok {
		return Prefix + h
	}
	return Prefix + name
}

// Serve serves GET Prefix<hashed name> with CacheControl; any other name,
// including an unhashed one, is a 404.
func Serve(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, Prefix)
	src, ok := served[name]
	if !ok {
		http.NotFound(w, r)
		return
	}
	f, err := src.fsys.Open(src.name)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	defer func() { _ = f.Close() }()
	content, ok := f.(io.ReadSeeker)
	if !ok {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", CacheControl)
	http.ServeContent(w, r, name, time.Time{}, content)
}
