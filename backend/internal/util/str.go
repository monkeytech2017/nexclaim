package util

// StrOr returns s if non-empty, otherwise def.
func StrOr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
