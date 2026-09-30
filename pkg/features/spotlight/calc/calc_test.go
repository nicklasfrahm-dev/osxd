package calc

import "testing"

func TestEvaluate(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"1+2", "3"},
		{"2 + 3 * 4", "14"},
		{"(2 + 3) * 4", "20"},
		{"10 / 4", "2.5"},
		{"7 % 3", "1"},
		{"2^10", "1024"},
		{"2**3**2", "512"},
		{"-2^2", "-4"},
		{"2^-1", "0.5"},
		{"3 - -2", "5"},
		{"0.1 + 0.2", "0.3"},
		{"6 × 7", "42"},
		{"9 ÷ 3", "3"},
		{"1.5e3 * 2", "3000"},
		{"sqrt(16)", "4"},
		{"2 * pi", "6.28318530718"},
		{"= 1 + 1", "2"},
		{"-(1 - 1)", "0"},
		{"1/3", "0.333333333333"},
	} {
		got, ok := Evaluate(tc.in)
		if !ok || got != tc.want {
			t.Errorf("Evaluate(%q) = %q, %v; want %q", tc.in, got, ok, tc.want)
		}
	}
}

func TestEvaluateRejects(t *testing.T) {
	for _, in := range []string{
		"", "42", "-5", "pi", "e", // not calculations
		"firefox", "vsc", "2 +", "(1 + 2", "1 / 0", "5 % 0", "foo(2)", "sqrt 4", "sqrt(-1)",
	} {
		if got, ok := Evaluate(in); ok {
			t.Errorf("Evaluate(%q) = %q, want no result", in, got)
		}
	}
}
