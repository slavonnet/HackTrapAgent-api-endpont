package filter

type Rule map[string]string

func MatchesAny(values map[string]string, rules []Rule) bool {
	for _, rule := range rules {
		if matchesRule(values, rule) {
			return true
		}
	}
	return false
}

func matchesRule(values map[string]string, rule Rule) bool {
	if len(rule) == 0 {
		return false
	}

	for key, expected := range rule {
		actual, ok := values[key]
		if !ok || actual != expected {
			return false
		}
	}

	return true
}
