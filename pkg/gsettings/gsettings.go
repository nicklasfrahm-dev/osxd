// Package gsettings holds helpers for changing GNOME settings through the
// gsettings command.
package gsettings

import (
	"fmt"
	"os/exec"
	"strings"
)

// Runner executes `gsettings args...` and returns trimmed stdout.
type Runner func(args ...string) (string, error)

// Run is the Runner that calls the real gsettings command.
func Run(args ...string) (string, error) {
	out, err := exec.Command("gsettings", args...).Output()
	return strings.TrimSpace(string(out)), err
}

// ParseList parses a gsettings string array such as `['a', 'b']` or `@as []`.
func ParseList(s string) []string {
	var out []string
	for {
		i := strings.IndexByte(s, '\'')
		if i < 0 {
			return out
		}
		j := strings.IndexByte(s[i+1:], '\'')
		if j < 0 {
			return out
		}
		out = append(out, s[i+1:i+1+j])
		s = s[i+j+2:]
	}
}

// FormatList formats items as a gsettings string array.
func FormatList(items []string) string {
	if len(items) == 0 {
		return "@as []"
	}
	q := make([]string, len(items))
	for i, it := range items {
		q[i] = fmt.Sprintf("'%s'", it)
	}
	return "[" + strings.Join(q, ", ") + "]"
}
