package resource

import "errors"

var (
	ErrInvalidArgument            = errors.New("invalid argument")
	ErrCantFindGatewayServersHost = errors.New("can't find Istio Gateway servers hosts")
)
