package rules

import (
	"fmt"
	"regexp"
	"strings"
)

type Action string

const (
	ActionIgnore Action = "ignore"
	ActionSend   Action = "send"
)

type rule struct {
	regex               *regexp.Regexp
	groupNamesDecorated []string
	action              Action
}

type Rules []rule

func (rs *Rules) Apply(topic string) (Action, *strings.Replacer) {
	a := ActionSend
	for _, r := range *rs {
		if r.regex == nil {
			a = r.action
			continue
		}

		ms := r.regex.FindStringSubmatch(topic)
		if ms == nil {
			continue
		}

		oldNew := make([]string, 0, len(r.groupNamesDecorated)*2)

		for ix, name := range r.groupNamesDecorated {
			oldNew = append(oldNew, name)
			oldNew = append(oldNew, ms[ix+1])
		}

		return r.action, strings.NewReplacer(oldNew...)
	}

	return a, nil
}

func ParseRules(texts []string) (Rules, error) {
	ret := make(Rules, 0, len(texts))

	for _, text := range texts {
		var r rule
		var err error

		ix := strings.Index(text, "->")
		if ix == -1 {
			r.action, err = parseAction(text)
		} else {
			r.regex, err = regexp.Compile("^" + text[:ix] + "$")
			if err != nil {
				return nil, fmt.Errorf("failed to compile regex: %w", err)
			}

			names := r.regex.SubexpNames()
			r.groupNamesDecorated = make([]string, 0, len(names)-1)
			for _, name := range names[1:] {
				r.groupNamesDecorated = append(r.groupNamesDecorated, "{"+name+"}")
			}

			r.action, err = parseAction(text[ix+2:])
		}

		if err != nil {
			return nil, fmt.Errorf("parsing action: %w", err)
		}

		ret = append(ret, r)
	}

	return ret, nil
}

func parseAction(text string) (Action, error) {
	switch text {
	case string(ActionIgnore):
		return ActionIgnore, nil
	case string(ActionSend):
		return ActionSend, nil
	default:
		return "", fmt.Errorf("unknown action: %s", text)
	}
}
