package random

import (
	"math/rand"

	"k8s.io/apimachinery/pkg/types"
)

const (
	randomNameLength        = 8
	randomNameCharSet       = "abcdefghijklmnopqrstuvwxyz"
	randomPortUpperBoundary = 65535
)

// Name creates a random string [a-z] of len 8.
func Name() string {
	b := make([]byte, randomNameLength)
	for i := range b {
		//nolint:gosec // random number generator sufficient for testing purposes
		b[i] = randomNameCharSet[rand.Intn(len(randomNameCharSet))]
	}
	return string(b)
}

func NamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Name:      Name(),
		Namespace: Name(),
	}
}

// Port creates a random int64 in range [1, 65535].
func Port() int64 {
	//nolint:gosec // random number generator sufficient for testing purposes
	return int64(rand.Intn(randomPortUpperBoundary) + 1)
}
