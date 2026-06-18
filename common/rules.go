package common

import (
	"regexp"

	log "github.com/sirupsen/logrus"
)

// MatchWindowRule returns the first window_rule whose Class regex matches the
// window's WM_CLASS and (when set) whose Name regex matches the WM_NAME.
// First match wins, in declaration order. Returns nil when no rule matches.
func MatchWindowRule(class, name string) *WindowRule {
	for i := range Config.WindowRules {
		r := &Config.WindowRules[i]
		if r.Class == "" {
			continue
		}
		if !matchRegex(r.Class, class) {
			continue
		}
		if r.Name != "" && !matchRegex(r.Name, name) {
			continue
		}
		return r
	}
	return nil
}

// MatchWorkspaceRule looks up a rule for the given 0-indexed bsptile desktop
// and screen. Rules in the config are 1-indexed (user-facing), so we add one
// before comparing. A rule with no Screen set applies to every screen of its
// desktop. First match wins.
func MatchWorkspaceRule(desktop, screen uint) *WorkspaceRule {
	wantDesktop := int(desktop) + 1
	wantScreen := int(screen) + 1
	for i := range Config.WorkspaceRules {
		r := &Config.WorkspaceRules[i]
		if r.Desktop != wantDesktop {
			continue
		}
		if r.Screen != nil && *r.Screen != wantScreen {
			continue
		}
		return r
	}
	return nil
}

func matchRegex(pattern, value string) bool {
	if pattern == "" {
		return false
	}
	matched, err := regexp.MatchString(pattern, value)
	if err != nil {
		log.Warn("Invalid window rule regex ", pattern, ": ", err)
		return false
	}
	return matched
}
