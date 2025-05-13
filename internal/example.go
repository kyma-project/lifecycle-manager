package internal

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SomeService struct {
	manifestClient manifestClient
}

type manifestClient interface {
	client.Writer
	client.Reader
}
