// +build !solution

package lab1

import "strconv"

// Errors is an error returned by a multiwriter when there are errors with one
// or more of the writes. Errors will be in a one-to-one correspondence with
// the input elements; successful elements will have a nil entry.
//
// Should not be constructed if all entires are nil; in this case one should
// instead return just nil instead of a MultiError.
type Errors []error

/*
Task 4: Errors needed for multiwriter

You may find this blog post useful:
http://blog.golang.org/error-handling-and-go

Similar to a the Stringer interface, the error interface also defines a
method that returns a string.

type error interface {
    Error() string
}

Thus also the error type can describe itself as a string. The fmt package (and
many others) use this Error() method to print errors.

Implement the Error() method for the Errors type defined above.

For the following conditions should be covered.

1. When there are no errors in the slice, it should return:

"(0 errors)"

2. When there is one error in the slice, it should return:

The error string return by the corresponding Error() method.

3. When there are two errors in the slice, it should return:

The first error + " (and 1 other error)"

4. When there are X>1 errors in the slice, it should return:

The first error + " (and X other errors)"
*/
func (m Errors) Error() string {

	noNil := Errors{}

	for i := 0; i < len(m); i++ {
		if m[i] != nil {
			noNil = append(noNil, m[i]) //I trust that m[i] is of type error or an errors zero-value, which is nil.
		}
	}

	l := len(noNil)
	switch {
	case l == 0:
		return "(0 errors)"
	case l == 1:
		return noNil[0].Error()
	case l == 2:
		return noNil[0].Error() + " (and 1 other error)"
	case l > 2:
		return noNil[0].Error() + " (and " + strconv.Itoa(l-1) + " other errors)"
	}

	return ""
}
