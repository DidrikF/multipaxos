package lab1

import (
	"strconv"
)

/*
Task 2: Stringers

One of the most ubiquitous interfaces is Stringer defined by the fmt package.

type Stringer interface {
    String() string
}

A Stringer is a type that can describe itself as a string. The fmt package (and
many others) look for this interface to print values.

Implement the String() method for the Student struct.

A struct

Student{ID: 42, FirstName: John, LastName: Doe, Age: 25}

should be printed as

"Student ID: 42. Name: Doe, John. Age: 25.
*/

type Student struct {
	ID        int
	FirstName string
	LastName  string
	Age       int
}

func (s Student) String() string {
	return "Student ID: " + strconv.Itoa(s.ID) + ". Name: " + s.LastName + ", " + s.FirstName + ". Age: " + strconv.Itoa(s.Age) + "."
}
