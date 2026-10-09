package store

import "strings"

func rebindForSQLite(query string) string {
	var out strings.Builder
	out.Grow(len(query))
	inString := false
	for i := 0; i < len(query); i++ {
		c := query[i]
		switch {
		case c == '\'':
			inString = !inString
			out.WriteByte(c)
		case c == '$' && !inString && i+1 < len(query) && isDigit(query[i+1]):
			out.WriteByte('?')
		default:
			out.WriteByte(c)
		}
	}
	return out.String()
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}
