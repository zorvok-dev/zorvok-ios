// Package cdn implements the CocoaPods "CDN source" protocol in front of
// registry-server/packages, so a Zorvok registry can be used directly as
// a CocoaPods Podfile `source` URL — the piece ADR 0005's native
// installer needs to make `zorvok install --native --source
// <zorvok-registry-url>/cdn/` actually work against a real Zorvok
// registry, not just a hand-built local Specs repo fixture.
//
// The protocol itself is not guessed or inferred from docs — it's taken
// directly from the installed `cocoapods`/`cocoapods-core` gem source
// (`Pod::Source::Manager#cdn_url?`, `Pod::CDNSource`,
// `Pod::Source::Metadata#path_fragment`), read before writing this
// package. Four things a CDN source must serve, verified against that
// source:
//
//  1. GET /CocoaPods-version.yml — how `cdn_url?` decides a fresh
//     Podfile `source` URL is a CDN (not something to `git clone`), and
//     how `Source::Metadata` learns the shard `prefix_lengths`.
//  2. GET /all_pods.txt — full pod name listing (Source#pods).
//  3. GET /all_pods_versions.txt — this registry sets `prefix_lengths: []`
//     (no sharding — Zorvok's own package count doesn't remotely need
//     cdn.cocoapods.org's 3-level MD5 sharding), so
//     Metadata#path_fragment always returns an empty shard fragment and
//     CDNSource always asks for this exact, unsharded filename (see
//     `index_file_name_for_fragment` in cdn_source.rb: an empty
//     fragment list means no `_<fragment>` suffix at all).
//  4. GET /Specs/<name>/<version>/<name>.podspec.json — the actual
//     podspec, stored verbatim (per packages.Version.Podspec) and
//     served verbatim; same "never execute it" rule as everywhere else
//     in this registry.
package cdn

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/zorvok-dev/zorvok/registry-server/packages"
)

// cocoaPodsVersion is the CocoaPods version this registry declares
// itself compatible with in CocoaPods-version.yml's `last` field —
// `cdn_url?` only requires this to be present and parseable, not that
// it matches the client's own installed version.
const cocoaPodsVersion = "1.16.2"

// Handler serves the CDN protocol for pkgs. Mount it under a path
// prefix (e.g. "/cdn/") with http.StripPrefix — every route here is
// relative to that mount point, matching how a Podfile `source` line
// points at the mount point itself (e.g. "http://host/cdn/").
func Handler(pkgs packages.Store) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /CocoaPods-version.yml", handleVersionYAML)
	mux.HandleFunc("GET /all_pods.txt", handleAllPods(pkgs))
	mux.HandleFunc("GET /all_pods_versions.txt", handleAllPodsVersions(pkgs))
	mux.HandleFunc("GET /Specs/{name}/{version}/{filename}", handlePodspec(pkgs))
	return mux
}

func handleVersionYAML(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/yaml")
	// prefix_lengths: [] disables sharding entirely — see the package
	// doc comment. Hand-written, not YAML-marshaled: the shape is fixed
	// and trivial enough that a library adds risk (subtle array/flow
	// formatting differences) without adding safety.
	fmt.Fprintf(w, "min: '0.0.1'\nlast: '%s'\nprefix_lengths: []\n", cocoaPodsVersion)
}

func handleAllPods(pkgs packages.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		all, err := pkgs.Search(r.Context(), "")
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		for _, p := range all {
			fmt.Fprintln(w, p.Name)
		}
	}
}

func handleAllPodsVersions(pkgs packages.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		all, err := pkgs.Search(r.Context(), "")
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		for _, p := range all {
			versions, err := pkgs.ListVersions(r.Context(), p.Name)
			if err != nil || len(versions) == 0 {
				continue
			}
			parts := make([]string, 0, len(versions)+1)
			parts = append(parts, p.Name)
			for _, v := range versions {
				parts = append(parts, v.Version)
			}
			// Format: "<PodName>/<v1>/<v2>/..." — parsed by CDNSource as
			// `row.split('/'); pod = row.shift; versions = row`. Sort
			// order doesn't matter: CDNSource re-sorts versions itself
			// after parsing (cdn_source.rb's `versions` method).
			fmt.Fprintln(w, strings.Join(parts, "/"))
		}
	}
}

func handlePodspec(pkgs packages.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		version := r.PathValue("version")
		filename := r.PathValue("filename")

		if filename != name+".podspec.json" {
			// A CDN source only ever requests "<name>/<version>/<name>.podspec.json"
			// (see CDNSource#specification_path); anything else is not a
			// request our protocol implementation produces, so it's not a
			// pod we recognize under that name.
			http.NotFound(w, r)
			return
		}

		v, err := pkgs.GetVersion(r.Context(), name, version)
		if err != nil {
			http.NotFound(w, r) // includes "pod not on this registry" — the federation fallthrough case (ADR 0003)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, v.Podspec)
	}
}
