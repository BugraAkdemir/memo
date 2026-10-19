// SPDX-License-Identifier: AGPL-3.0-or-later

package project

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// SkillSource names a skill on GitHub.
type SkillSource struct {
	Owner string
	Repo  string
	// Ref is a branch, tag or commit; empty means the repository's default branch.
	Ref string
	// Path is a folder inside the repository to look in; empty means the whole repo.
	Path string
	// Skill selects one skill by name when the place holds several.
	Skill string
}

func (s SkillSource) String() string {
	out := s.Owner + "/" + s.Repo
	if s.Path != "" {
		out += "/" + s.Path
	}
	if s.Ref != "" {
		out += "@" + s.Ref
	}
	if s.Skill != "" {
		out += " (" + s.Skill + ")"
	}
	return out
}

var (
	ownerRepoRe = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
	pathPartRe  = regexp.MustCompile(`^[A-Za-z0-9_.@+ -]+$`)

	// Words that only start the runner or the sub-command; they carry no source.
	runnerWords = map[string]bool{
		"npx": true, "bunx": true, "pnpx": true, "pnpm": true, "yarn": true, "npm": true,
		"dlx": true, "exec": true, "-y": true, "--yes": true, "--no-install": true,
		"add": true, "install": true, "i": true,
	}
	// Flags that take a value we do not use.
	ignoredValueFlags = map[string]bool{"-a": true, "--agent": true, "--branch": true}
)

// ErrSkillSource is wrapped by every ParseSkillSource failure.
var ErrSkillSource = errors.New("skill source")

// ParseSkillSource understands what a person pastes when they want a skill:
//
//	owner/repo
//	owner/repo/some/folder
//	https://github.com/owner/repo
//	https://github.com/owner/repo/tree/main/skills/foo
//	github:owner/repo      git@github.com:owner/repo.git
//	npx skills add owner/repo --skill foo        (and bunx / pnpm dlx / -y variants)
//
// The command forms are only *read*: Memo never runs npx (or anything else) —
// it takes the source out of the line and downloads the files itself. Only
// GitHub is supported; anything else is refused with a clear message.
func ParseSkillSource(input string) (SkillSource, error) {
	var (
		src        SkillSource
		positional []string
		fields     = strings.Fields(strings.TrimSpace(input))
	)
	if len(fields) == 0 {
		return src, fmt.Errorf("%w: nothing to install from", ErrSkillSource)
	}
	for i := 0; i < len(fields); i++ {
		f := strings.Trim(fields[i], `"'`)
		low := strings.ToLower(f)
		switch {
		case f == "":
		case runnerWords[low], low == "skills", strings.HasPrefix(low, "skills@"), low == "add-skill", strings.HasPrefix(low, "add-skill@"):
			// the runner / sub-command words
		case f == "--skill" || f == "-s":
			if i+1 < len(fields) {
				i++
				src.Skill = strings.Trim(fields[i], `"'`)
			}
		case strings.HasPrefix(f, "--skill="):
			src.Skill = strings.Trim(strings.TrimPrefix(f, "--skill="), `"'`)
		case ignoredValueFlags[f]:
			i++ // skip the flag's value
		case strings.HasPrefix(f, "-"):
			// any other flag (-g, --global, --yes…) has no meaning here
		default:
			positional = append(positional, f)
		}
	}
	switch len(positional) {
	case 0:
		return src, fmt.Errorf("%w: no source found in %q", ErrSkillSource, input)
	case 1:
	default:
		return src, fmt.Errorf("%w: expected one source, got %d (%s)", ErrSkillSource, len(positional), strings.Join(positional, " "))
	}

	if err := parseGitHubRef(positional[0], &src); err != nil {
		return SkillSource{}, err
	}
	if src.Skill != "" && !validSkillName(src.Skill) {
		return SkillSource{}, fmt.Errorf("%w: %q is not a valid skill name", ErrSkillSource, src.Skill)
	}
	return src, nil
}

func parseGitHubRef(raw string, src *SkillSource) error {
	s := raw
	switch {
	case strings.HasPrefix(s, "git@github.com:"):
		s = strings.TrimPrefix(s, "git@github.com:")
	case strings.HasPrefix(s, "github:"):
		s = strings.TrimPrefix(s, "github:")
	case strings.HasPrefix(s, "https://github.com/"), strings.HasPrefix(s, "http://github.com/"),
		strings.HasPrefix(s, "https://www.github.com/"):
		if i := strings.Index(s, "github.com/"); i >= 0 {
			s = s[i+len("github.com/"):]
		}
	case strings.Contains(s, "://") || strings.Contains(s, "@") && strings.Contains(s, ":"):
		return fmt.Errorf("%w: only GitHub sources are supported (got %q)", ErrSkillSource, raw)
	}
	s = strings.TrimSuffix(strings.SplitN(s, "?", 2)[0], "/")
	parts := strings.Split(s, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("%w: expected owner/repo, got %q", ErrSkillSource, raw)
	}
	src.Owner = parts[0]
	src.Repo = strings.TrimSuffix(parts[1], ".git")
	if !ownerRepoRe.MatchString(src.Owner) || !ownerRepoRe.MatchString(src.Repo) || src.Repo == "" {
		return fmt.Errorf("%w: %q is not a valid GitHub owner/repo", ErrSkillSource, raw)
	}
	rest := parts[2:]
	// https://github.com/o/r/tree/<ref>/<path…> — a ref with a slash in it
	// cannot be told apart from a path here; the first segment is the ref.
	if len(rest) >= 2 && (rest[0] == "tree" || rest[0] == "blob") {
		src.Ref = rest[1]
		rest = rest[2:]
	} else if len(rest) == 1 && rest[0] == "tree" {
		rest = nil
	}
	for _, p := range rest {
		if p == "" || p == "." || p == ".." || !pathPartRe.MatchString(p) {
			return fmt.Errorf("%w: unsafe path segment %q", ErrSkillSource, p)
		}
	}
	if src.Ref != "" && !pathPartRe.MatchString(src.Ref) {
		return fmt.Errorf("%w: unsafe ref %q", ErrSkillSource, src.Ref)
	}
	src.Path = strings.Join(rest, "/")
	return nil
}

var skillNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// validSkillName is what a skill's folder under .memo/skills may be called: a
// single safe path component.
func validSkillName(name string) bool {
	return skillNameRe.MatchString(name) && !strings.Contains(name, "..")
}
