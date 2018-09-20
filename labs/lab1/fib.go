package lab1 // +build !solution

// Task 1: Fibonacci numbers
//
// fibonacci(n) returns the n-th Fibonacci number, and is defined by the
// recurrence relation F_n = F_n-1 + F_n-2, with seed values F_0=0 and F_1=1.
func fibonacci(n uint) uint { //uint
	var F_0 uint = 0
	var F_1 uint = 1
	numbers := []uint{F_0, F_1}

	for i := uint(0); i <= n; i++ {
		numbers = append(numbers, numbers[i]+numbers[i+1])
	}

	return numbers[n]
}
