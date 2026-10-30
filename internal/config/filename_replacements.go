package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"regexp/syntax"
	"sort"
	"strings"
)

// FilenameReplacement is one ordered replacement. Regex records rules that must
// stay intact (and inactive) when regex mode is turned off.
type FilenameReplacement struct {
	Search      string `json:"search"`
	Replacement string `json:"replacement"`
	Regex       bool   `json:"regex,omitempty"`
}

type FilenameReplacements []FilenameReplacement

// UnmarshalJSON accepts the original literal map as well as ordered rules.
func (rules *FilenameReplacements) UnmarshalJSON(data []byte) error {
	if len(bytes.TrimSpace(data)) > 0 && bytes.TrimSpace(data)[0] == '{' {
		var legacy map[string]string
		if err := json.Unmarshal(data, &legacy); err != nil {
			return err
		}
		keys := make([]string, 0, len(legacy))
		for key := range legacy {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		*rules = make(FilenameReplacements, 0, len(keys))
		for _, key := range keys {
			*rules = append(*rules, FilenameReplacement{Search: key, Replacement: legacy[key]})
		}
		return nil
	}
	type plainRules FilenameReplacements
	return json.Unmarshal(data, (*plainRules)(rules))
}

// ValidateFilenameReplacement validates a row using Go's actual regex compiler.
func ValidateFilenameReplacement(rule FilenameReplacement, regexMode bool) error {
	if rule.Search == "" {
		return fmt.Errorf("search text must not be empty")
	}
	if regexMode || rule.Regex {
		if _, err := regexp.Compile(rule.Search); err != nil {
			return err
		}
	}
	for _, r := range rule.Replacement {
		if r < 32 || r == 127 || strings.ContainsRune(`<>:"/\|?*`, r) {
			return fmt.Errorf("replacement contains invalid filename character %q", r)
		}
	}
	return nil
}

func ValidateFilenameReplacements(rules FilenameReplacements, regexMode bool) error {
	for i, rule := range rules {
		if err := ValidateFilenameReplacement(rule, regexMode); err != nil {
			return fmt.Errorf("filename_replacements[%d]: %w", i+1, err)
		}
	}
	return nil
}

// AsRegex quotes literal searches and dollars in replacement text. Existing
// regex rules are left byte-for-byte intact.
func (rule FilenameReplacement) AsRegex() FilenameReplacement {
	if !rule.Regex {
		rule.Search = regexp.QuoteMeta(rule.Search)
		rule.Replacement = strings.ReplaceAll(rule.Replacement, "$", "$$")
		rule.Regex = true
	}
	return rule
}

// AsLiteral converts only patterns that represent one exact literal string and
// replacements without capture references. Conversion must also round-trip
// byte-for-byte; Go's parser can simplify intentional syntax such as [A] or \x41.
func (rule FilenameReplacement) AsLiteral() FilenameReplacement {
	if !rule.Regex {
		return rule
	}
	pattern, err := syntax.Parse(rule.Search, syntax.Perl)
	if err != nil || pattern.Op != syntax.OpLiteral || pattern.Flags&syntax.FoldCase != 0 {
		return rule
	}
	var replacement strings.Builder
	for i := 0; i < len(rule.Replacement); i++ {
		if rule.Replacement[i] == '$' {
			if i+1 >= len(rule.Replacement) || rule.Replacement[i+1] != '$' {
				return rule
			}
			i++
		}
		replacement.WriteByte(rule.Replacement[i])
	}
	literal := FilenameReplacement{Search: string(pattern.Rune), Replacement: replacement.String()}
	if literal.AsRegex() != rule {
		return rule
	}
	return literal
}

// ApplyFilenameReplacements preserves the original literal, non-cascading final
// name replacements. Regex mode instead preprocesses source names before parsing.
func (cfg *FormatConfig) ApplyFilenameReplacements(name string) string {
	if cfg.FilenameReplacementsRegex || len(cfg.FilenameReplacements) == 0 {
		return name
	}
	rules := append(FilenameReplacements(nil), cfg.FilenameReplacements...)
	sort.SliceStable(rules, func(i, j int) bool {
		return len(rules[i].Search) > len(rules[j].Search)
	})
	pairs := make([]string, 0, 2*len(rules))
	for _, rule := range rules {
		if rule.Search != "" && !rule.Regex {
			pairs = append(pairs, rule.Search, rule.Replacement)
		}
	}
	if len(pairs) == 0 {
		return name
	}
	return strings.Join(strings.Fields(strings.NewReplacer(pairs...).Replace(name)), " ")
}

// FilenamePreprocessor compiles the ordered pipeline once for use across a tree.
// Load/Save reject invalid rules; invalid in-memory rules are safely skipped.
func (cfg *FormatConfig) FilenamePreprocessor() func(string) string {
	type compiledRule struct {
		pattern     *regexp.Regexp
		replacement string
	}
	var compiled []compiledRule
	if cfg.FilenameReplacementsRegex {
		for _, rule := range cfg.FilenameReplacements {
			pattern, err := regexp.Compile(rule.Search)
			if err == nil && rule.Search != "" {
				compiled = append(compiled, compiledRule{pattern, rule.Replacement})
			}
		}
	}
	return func(name string) string {
		for _, rule := range compiled {
			name = rule.pattern.ReplaceAllString(name, rule.replacement)
		}
		return name
	}
}
