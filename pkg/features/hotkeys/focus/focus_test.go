package focus

import "testing"

func TestIsTerminal(t *testing.T) {
	tests := []struct {
		class string
		extra []string
		want  bool
	}{
		{"org.gnome.Ptyxis", nil, true},
		{"kitty", nil, true},
		{"Alacritty", nil, true},
		{"firefox", nil, false},
		{"code", nil, false},
		{"", nil, false},
		{"Cool-Retro-Term", []string{"cool-retro-term"}, true},
	}
	for _, tt := range tests {
		if got := IsTerminal(tt.class, tt.extra); got != tt.want {
			t.Errorf("IsTerminal(%q, %v) = %v, want %v", tt.class, tt.extra, got, tt.want)
		}
	}
}
