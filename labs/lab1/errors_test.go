package lab1

import (
	. "io"
	"testing"
)

// String: "io: read/write on closed pipe"
var s = ErrShortWrite.Error()

var errTests = []struct {
	in   Errors
	want string
}{
	{nil, "(0 errors)"},
	{[]error{}, "(0 errors)"},
	{[]error{ErrShortWrite}, s},
	{[]error{ErrShortWrite, ErrClosedPipe}, s + " (and 1 other error)"},
	{[]error{ErrShortWrite, ErrClosedPipe, ErrClosedPipe}, s + " (and 2 other errors)"},
	{[]error{nil}, "(0 errors)"},
	{[]error{ErrShortWrite, nil}, s},
	{[]error{nil, ErrShortWrite}, s},
	{[]error{ErrShortWrite, ErrClosedPipe, nil}, s + " (and 1 other error)"},
	{[]error{ErrShortWrite, ErrClosedPipe, nil, nil}, s + " (and 1 other error)"},
	{[]error{ErrShortWrite, ErrClosedPipe, nil, nil, nil}, s + " (and 1 other error)"},
	{[]error{nil, ErrShortWrite, nil, ErrClosedPipe}, s + " (and 1 other error)"},
	{[]error{nil, nil, ErrShortWrite, ErrClosedPipe}, s + " (and 1 other error)"},
	{[]error{ErrShortWrite, nil, nil, ErrClosedPipe, ErrClosedPipe}, s + " (and 2 other errors)"},
	{[]error{ErrShortWrite, ErrClosedPipe, nil, ErrClosedPipe, nil}, s + " (and 2 other errors)"},
	{[]error{nil, nil, nil, nil, ErrShortWrite, ErrClosedPipe, ErrClosedPipe}, s + " (and 2 other errors)"},
}

func TestErrors(t *testing.T) {
	for i, ft := range errTests {
		out := ft.in.Error()
		if out != ft.want {
			t.Errorf("Errors test %d: got %q for input %v, want %q", i, out, ft.in, ft.want)
		}
	}
}
