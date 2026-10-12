// Package conventional parses Conventional Commits messages into the small
// amount of structure release automation needs: type, scope, whether the change
// is breaking, and the resulting semver bump. Stdlib-only by design.
//
// See https://www.conventionalcommits.org for the spec this implements.
package conventional

import (
	"regexp"
	"strings"

	"github.com/lassediercksAI/releasejo/internal/semver"
)

// Commit is a parsed commit message. When the header does not match the
// Conventional Commits grammar, Type is empty and IsConventional reports false;
// the raw text is still preserved so callers can fall back gracefully.
type Commit struct {
	Type        string // lowercased commit type, e.g. "feat","fix","chore"; "" if the message is not a conventional commit
	Scope       string // optional scope from feat(scope):
	Breaking    bool   // true if header has "!" before ":" OR body/footer has a "BREAKING CHANGE:" / "BREAKING-CHANGE:" line
	Description string // summary text after the "type(scope): " prefix (first line)
	Body        string // everything after the first line, trimmed ("" if none)
	ReleaseAs   string // raw value of a "Release-As:" footer, leading "v" stripped; "" if none. NOT validated here — callers parse it as semver.
	Hash        string // the commit sha passed in
	Raw         string // the raw message passed in
}

// headerRe matches a Conventional Commits header line:
//
//	<type>(<scope>)!: <description>
//
// Groups: 1=type, 3=scope (inner, without parens), 4="!" marker, 5=description.
// The scope and "!" are optional. (?i) makes the whole pattern case-insensitive
// so "Feat:" parses; we normalise Type to lower-case ourselves afterwards.
var headerRe = regexp.MustCompile(`(?i)^(\w+)(\(([^)]+)\))?(!)?: (.+)$`)

// breakingRe matches a footer/body line that declares a breaking change. Per the
// spec the token is upper-case only ("BREAKING CHANGE" or "BREAKING-CHANGE"), so
// this pattern is intentionally case-sensitive — a lowercase "breaking change:"
// is prose, not a declaration.
var breakingRe = regexp.MustCompile(`^BREAKING[ -]CHANGE:`)

// releaseAsRe matches a "Release-As:" footer line, which pins the next release
// to an explicit version. Unlike BREAKING CHANGE, release-please accepts the key
// in any case ("Release-As", "release-as", "RELEASE-AS"), so this pattern is
// case-insensitive. Group 2 is the value with an optional leading "v" dropped;
// whether it is a usable version is decided by the caller, not here.
var releaseAsRe = regexp.MustCompile(`(?i)^release-as:[ \t]*(v)?(.+)$`)

// Parse extracts commit metadata from a raw message. hash is stored verbatim on
// the returned Commit. A message whose first line is not a valid Conventional
// Commit header yields a Commit with Type=="" (see IsConventional).
func Parse(message, hash string) Commit {
	// Normalise line endings so CRLF input parses like LF, then split the
	// header (first line) from the rest.
	normalized := strings.ReplaceAll(message, "\r\n", "\n")

	header := normalized
	rest := ""
	if i := strings.IndexByte(normalized, '\n'); i >= 0 {
		header = normalized[:i]
		rest = normalized[i+1:]
	}
	// Trim trailing whitespace/CR from the header so a stray "\r" or spaces
	// don't defeat the anchored regex.
	header = strings.TrimRight(header, " \t\r")

	body := strings.TrimSpace(rest)

	c := Commit{
		Hash:      hash,
		Raw:       message,
		Body:      body,
		ReleaseAs: releaseAsFooter(rest),
	}

	// A breaking change can be declared in the body/footer regardless of the
	// header shape, so scan for it up front on the raw (normalised) message.
	if hasBreakingFooter(normalized) {
		c.Breaking = true
	}

	m := headerRe.FindStringSubmatch(header)
	if m == nil {
		// Not a conventional commit: keep the header as the description so
		// callers still have something human-readable.
		c.Description = header
		return c
	}

	c.Type = strings.ToLower(m[1])
	c.Scope = m[3]
	c.Description = m[5]
	if m[4] == "!" {
		c.Breaking = true
	}

	return c
}

// hasBreakingFooter reports whether any line of msg is a "BREAKING CHANGE:" /
// "BREAKING-CHANGE:" declaration. Lines are right-trimmed so trailing CR from
// un-normalised input never hides a match.
func hasBreakingFooter(msg string) bool {
	for _, line := range strings.Split(msg, "\n") {
		if breakingRe.MatchString(strings.TrimRight(line, " \t\r")) {
			return true
		}
	}
	return false
}

// releaseAsFooter returns the value of a "Release-As:" footer in body (the
// message minus its header line), with any leading "v" and surrounding
// whitespace removed. The header is excluded on purpose: "fix: release-as: 1.2.3"
// is a subject line, not a footer. If several lines declare one, the last wins —
// a later footer overrides an earlier one, mirroring "the newest request wins"
// across commits.
func releaseAsFooter(body string) string {
	out := ""
	for _, line := range strings.Split(body, "\n") {
		if m := releaseAsRe.FindStringSubmatch(strings.TrimRight(line, " \t\r")); m != nil {
			out = strings.TrimSpace(m[2])
		}
	}
	return out
}

// IsConventional reports whether the message parsed as a Conventional Commit.
func (c Commit) IsConventional() bool { return c.Type != "" }

// Level returns the semver bump this commit implies: a breaking change is Major,
// "feat" is Minor, "fix"/"perf"/"revert" are Patch, and everything else (chore,
// docs, refactor, or a non-conventional message) is None — matching
// release-please's conventionalcommits preset, which releases on perf and revert
// too (a perf improvement or a revert is a shippable change, not just noise).
func (c Commit) Level() semver.Level {
	if c.Breaking {
		return semver.Major
	}
	switch c.Type {
	case "feat":
		return semver.Minor
	case "fix", "perf", "revert":
		return semver.Patch
	default:
		return semver.None
	}
}
