package istio

import "errors"

var (
	ErrCantFindMatchingGateway    = errors.New("can't find matching Istio Gateway")
	ErrInvalidArgument            = errors.New("invalid argument")
	ErrCantFindGatewayServersHost = errors.New("can't find Istio Gateway servers hosts")
)
