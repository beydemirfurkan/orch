package agents

// estimateChars returns a rough character count for the string, used for
// token estimation without importing a tokenizer. Assumes ~4 chars/token.
func estimateTokens(s string) int {
	if len(s) == 0 {
		return 0
	}
	t := len(s) / 4
	if t == 0 {
		return 1
	}
	return t
}

// truncateList returns at most maxItems entries from items, appending a
// "[+N more]" suffix entry when items were dropped.
func truncateList(items []string, maxItems int) []string {
	if maxItems <= 0 || len(items) <= maxItems {
		return items
	}
	dropped := len(items) - maxItems
	out := make([]string, maxItems+1)
	copy(out, items[:maxItems])
	out[maxItems] = "[+" + itoa(dropped) + " more omitted]"
	return out
}

// filterByBasename keeps only items whose base filename (without extension)
// appears in the allowed set. If allowed is empty, all items pass through.
func filterByBasename(items []string, allowedBases map[string]struct{}) []string {
	if len(allowedBases) == 0 {
		return items
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		base := baseNameNoExt(item)
		// Also try without trailing _test suffix (Go convention)
		trimmed := trimSuffix(trimPrefix(base, "test_"), "_test")
		if _, ok := allowedBases[base]; ok {
			out = append(out, item)
		} else if _, ok := allowedBases[trimmed]; ok {
			out = append(out, item)
		}
	}
	return out
}

func baseNameNoExt(path string) string {
	// find last slash
	start := 0
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			start = i + 1
			break
		}
	}
	name := path[start:]
	// strip extension
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '.' {
			return name[:i]
		}
	}
	return name
}

func trimSuffix(s, suffix string) string {
	if len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix {
		return s[:len(s)-len(suffix)]
	}
	return s
}

func trimPrefix(s, prefix string) string {
	if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):]
	}
	return s
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 10)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	return string(buf)
}
