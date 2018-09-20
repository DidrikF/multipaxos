// +build !solution

package lab1

import (
	"io"
)

/*
Task 3: Rot 13

This task is taken from http://tour.golang.org.

A common pattern is an io.Reader that wraps another io.Reader, modifying the
stream in some way.

For example, the gzip.NewReader function takes an io.Reader (a stream of
compressed data) and returns a *gzip.Reader that also implements io.Reader (a
stream of the decompressed data).

Implement a rot13Reader that implements io.Reader and reads from an io.Reader,
modifying the stream by applying the rot13 substitution cipher to all
alphabetical characters.

The rot13Reader type is provided for you. Make it an io.Reader by implementing
its Read method.
*/

type rot13Reader struct {
	r io.Reader //io.Reader is an interface wrapping a Read(p []byte) (n int, err error) method

	//This struct has an r property which requires a value that adheres to the io.Reader interface (has a Read method)

}

//you need to provide a byte slice to read into when calling this method
func (r rot13Reader) Read(p []byte) (n int, err error) { //this method is available to variables with a type of rot13Reader
	l, e := r.r.Read(p) //call the Read() method on the r property on the struct, which implements the io.Reader interface.

	for i := 0; i < l; i++ { //loop over each byte that was read into the byte slice "p".
		switch {
		case 'a' <= p[i] && p[i] <= 'z': //Unicode is a superset of ASCII, UTF8 is a way to store Unicode characters in a byte sequence. The number equivalient of the ASCII/UTF8 character is what is being compared to the number representation of the byte.
			p[i] = 'a' + (p[i]-'a'+13)%26 //shifting the character 13 places
		case 'A' <= p[i] && p[i] <= 'Z':
			p[i] = 'A' + (p[i]-'A'+13)%26
		}
	}

	return l, e //retun the result of the Read operation

	//slices are passed in by reference, so no need to return it.
}

/*
type Reader interface {
        Read(p []byte) (n int, err error)
}

*/
