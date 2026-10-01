package remap

import (
	"reflect"
	"testing"
)

const keyLeft uint16 = 105

func p(code uint16) Event { return Event{code, Press} }
func r(code uint16) Event { return Event{code, Release} }

func run(mode Mode, in ...Event) []Event {
	m := New(func() Mode { return mode })
	var out []Event
	for _, e := range in {
		out = append(out, m.Process(e)...)
	}
	return out
}

func TestRemap(t *testing.T) {
	tests := []struct {
		name string
		mode Mode
		in   []Event
		want []Event
	}{
		{
			name: "copy in app",
			mode: App,
			in:   []Event{p(KeyLeftMeta), p(KeyC), r(KeyC), r(KeyLeftMeta)},
			want: []Event{p(KeyLeftMeta), p(KeyLeftCtrl), r(KeyLeftMeta), p(KeyC), r(KeyC), r(KeyLeftCtrl)},
		},
		{
			name: "copy in terminal",
			mode: Terminal,
			in:   []Event{p(KeyLeftMeta), p(KeyC), r(KeyC), r(KeyLeftMeta)},
			want: []Event{
				p(KeyLeftMeta), p(KeyLeftCtrl), p(KeyLeftShift), r(KeyLeftMeta), p(KeyC),
				r(KeyC), r(KeyLeftCtrl), r(KeyLeftShift),
			},
		},
		{
			name: "paste in terminal",
			mode: Terminal,
			in:   []Event{p(KeyLeftMeta), p(KeyV), r(KeyV), r(KeyLeftMeta)},
			want: []Event{
				p(KeyLeftMeta), p(KeyLeftCtrl), p(KeyLeftShift), r(KeyLeftMeta), p(KeyV),
				r(KeyV), r(KeyLeftCtrl), r(KeyLeftShift),
			},
		},
		{
			name: "unknown window swallows the key",
			mode: Unknown,
			in:   []Event{p(KeyLeftMeta), p(KeyC), {KeyC, Repeat}, r(KeyC), r(KeyLeftMeta)},
			want: []Event{p(KeyLeftMeta), p(KeyLeftCtrl), r(KeyLeftMeta), r(KeyLeftCtrl)},
		},
		{
			name: "unknown window still allows other Super shortcuts",
			mode: Unknown,
			in:   []Event{p(KeyLeftMeta), p(KeyC), r(KeyC), p(keyLeft), r(keyLeft), r(KeyLeftMeta)},
			want: []Event{
				p(KeyLeftMeta), p(KeyLeftCtrl), r(KeyLeftMeta),
				p(KeyLeftMeta), r(KeyLeftCtrl), p(keyLeft), r(keyLeft), r(KeyLeftMeta),
			},
		},
		{
			name: "unmapped key keeps Super",
			mode: App,
			in:   []Event{p(KeyLeftMeta), p(keyLeft), r(keyLeft), r(KeyLeftMeta)},
			want: []Event{p(KeyLeftMeta), p(keyLeft), r(keyLeft), r(KeyLeftMeta)},
		},
		{
			name: "several shortcuts in one hold",
			mode: App,
			in:   []Event{p(KeyLeftMeta), p(KeyA), r(KeyA), p(KeyC), r(KeyC), r(KeyLeftMeta)},
			want: []Event{
				p(KeyLeftMeta), p(KeyLeftCtrl), r(KeyLeftMeta), p(KeyA), r(KeyA),
				p(KeyC), r(KeyC), r(KeyLeftCtrl),
			},
		},
		{
			name: "unmapped key after a shortcut swaps Super back",
			mode: App,
			in:   []Event{p(KeyLeftMeta), p(KeyC), r(KeyC), p(keyLeft), r(keyLeft), r(KeyLeftMeta)},
			want: []Event{
				p(KeyLeftMeta), p(KeyLeftCtrl), r(KeyLeftMeta), p(KeyC), r(KeyC),
				p(KeyLeftMeta), r(KeyLeftCtrl), p(keyLeft), r(keyLeft), r(KeyLeftMeta),
			},
		},
		{
			name: "shift joins the shortcut",
			mode: App,
			in:   []Event{p(KeyLeftMeta), p(KeyZ), r(KeyZ), p(KeyLeftShift), p(KeyZ), r(KeyZ), r(KeyLeftShift), r(KeyLeftMeta)},
			want: []Event{
				p(KeyLeftMeta), p(KeyLeftCtrl), r(KeyLeftMeta), p(KeyZ), r(KeyZ),
				p(KeyLeftShift), p(KeyZ), r(KeyZ), r(KeyLeftShift), r(KeyLeftCtrl),
			},
		},
		{
			name: "held Ctrl is not released",
			mode: App,
			in:   []Event{p(KeyLeftCtrl), p(KeyLeftMeta), p(KeyC), r(KeyC), r(KeyLeftMeta), r(KeyLeftCtrl)},
			want: []Event{p(KeyLeftCtrl), p(KeyLeftMeta), r(KeyLeftMeta), p(KeyC), r(KeyC), r(KeyLeftCtrl)},
		},
		{
			name: "Ctrl released during the swap stays down until Super is",
			mode: App,
			in:   []Event{p(KeyLeftMeta), p(KeyC), p(KeyLeftCtrl), r(KeyLeftCtrl), r(KeyC), r(KeyLeftMeta)},
			want: []Event{p(KeyLeftMeta), p(KeyLeftCtrl), r(KeyLeftMeta), p(KeyC), r(KeyC), r(KeyLeftCtrl)},
		},
		{
			name: "plain keys pass through",
			mode: App,
			in:   []Event{p(KeyC), {KeyC, Repeat}, r(KeyC)},
			want: []Event{p(KeyC), {KeyC, Repeat}, r(KeyC)},
		},
		{
			name: "Super does not repeat while swapped",
			mode: App,
			in:   []Event{p(KeyLeftMeta), p(KeyC), {KeyLeftMeta, Repeat}, {KeyC, Repeat}, r(KeyC), r(KeyLeftMeta)},
			want: []Event{p(KeyLeftMeta), p(KeyLeftCtrl), r(KeyLeftMeta), p(KeyC), {KeyC, Repeat}, r(KeyC), r(KeyLeftCtrl)},
		},
		{
			name: "both Super keys",
			mode: App,
			in:   []Event{p(KeyLeftMeta), p(KeyRightMeta), p(KeyS), r(KeyS), r(KeyLeftMeta), r(KeyRightMeta)},
			want: []Event{
				p(KeyLeftMeta), p(KeyRightMeta), p(KeyLeftCtrl), r(KeyLeftMeta), r(KeyRightMeta),
				p(KeyS), r(KeyS), r(KeyLeftCtrl),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(tt.mode, tt.in...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got  %v\nwant %v", got, tt.want)
			}
		})
	}
}

func TestModeAskedOncePerSwap(t *testing.T) {
	calls := 0
	m := New(func() Mode { calls++; return App })
	for _, e := range []Event{p(KeyLeftMeta), p(KeyA), r(KeyA), p(KeyC), r(KeyC), r(KeyLeftMeta)} {
		m.Process(e)
	}
	if calls != 1 {
		t.Fatalf("Mode called %d times, want 1", calls)
	}
}
