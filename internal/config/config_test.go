package config

import "testing"

func TestParseConfigDefaultsMerge(t *testing.T) {
	raw := []byte(`{
      "release-type": "go",
      "bump-minor-pre-major": true,
      "include-v-in-tag": true,
      "tag-separator": "-",
      "packages": {
        ".": { "component": "root" },
        "services/api": { "release-type": "node", "component": "api" }
      }
    }`)
	cfg, err := ParseConfig(raw)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if len(cfg.Packages) != 2 {
		t.Fatalf("want 2 packages, got %d", len(cfg.Packages))
	}
	// sorted by path: "." then "services/api"
	root := cfg.Packages[0]
	if root.Path != "." || root.ReleaseType != "go" {
		t.Errorf("root package: got path=%q type=%q", root.Path, root.ReleaseType)
	}
	if !root.BumpMinorPreMajor {
		t.Error("root should inherit bump-minor-pre-major=true")
	}
	api := cfg.Packages[1]
	if api.ReleaseType != "node" {
		t.Errorf("api release-type override: got %q want node", api.ReleaseType)
	}
	if !api.BumpMinorPreMajor {
		t.Error("api should inherit bump-minor-pre-major=true")
	}
	// monorepo (>1 package) => component included by default
	if !cfg.IncludeComponent(api) {
		t.Error("expected include-component-in-tag defaulted true for monorepo")
	}
	if api.ChangelogPath != "CHANGELOG.md" {
		t.Errorf("default changelog path: %q", api.ChangelogPath)
	}
}

// release-as follows the same precedence as the other defaults: a top-level
// value applies to every package, a package-level value overrides it. The value
// is kept verbatim — validity is decided where the version is computed.
func TestParseConfigReleaseAs(t *testing.T) {
	cfg, err := ParseConfig([]byte(`{
      "release-type": "go",
      "release-as": "1.0.0",
      "packages": {
        ".": { "component": "root" },
        "services/api": { "component": "api", "release-as": "2.3.4" }
      }
    }`))
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if got := cfg.Packages[0].ReleaseAs; got != "1.0.0" {
		t.Errorf("root release-as = %q, want the top-level 1.0.0", got)
	}
	if got := cfg.Packages[1].ReleaseAs; got != "2.3.4" {
		t.Errorf("api release-as = %q, want the package override 2.3.4", got)
	}

	// unset anywhere => empty, i.e. no override
	cfg, err = ParseConfig([]byte(`{"release-type":"go","packages":{".":{}}}`))
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if got := cfg.Packages[0].ReleaseAs; got != "" {
		t.Errorf("release-as = %q, want empty when unset", got)
	}
}

func TestParseConfigErrors(t *testing.T) {
	if _, err := ParseConfig([]byte(`{"packages":{}}`)); err == nil {
		t.Error("empty packages should error")
	}
	if _, err := ParseConfig([]byte(`{"packages":{".":{}}}`)); err == nil {
		t.Error("missing release-type with no global default should error")
	}
}

func TestManifestRoundTrip(t *testing.T) {
	m := Manifest{".": "1.2.3", "services/api": "0.4.0"}
	b, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \".\": \"1.2.3\",\n  \"services/api\": \"0.4.0\"\n}\n"
	if string(b) != want {
		t.Errorf("Marshal =\n%q\nwant\n%q", b, want)
	}
}
