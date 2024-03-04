package random

import "math/rand"

const (
	randomNameLength  = 8
	randomNameCharSet = "abcdefghijklmnopqrstuvwxyz"
)

// RandomName creates a random string [a-z] of len 8.
func Name() string {
	b := make([]byte, randomNameLength)
	for i := range b {
		//nolint:gosec // random number generator sufficient for testing purposes
		b[i] = randomNameCharSet[rand.Intn(len(randomNameCharSet))]
	}
	return string(b)
}
