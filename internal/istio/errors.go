package istio

import "errors"

var (
	ErrFailedToCreateIstioClient         = errors.New("failed to create istio client from config")
	ErrFailedToGetVirtualService         = errors.New("failed to get virtual service")
	ErrFailedToListVirtualServices       = errors.New("failed to list virtual services")
	ErrFailedToCreateVirtualService      = errors.New("failed to create virtual service")
	ErrFailedToUpdateVirtualService      = errors.New("failed to update virtual service")
	ErrFailedToDeleteVirtualService      = errors.New("failed to delete virtual service")
	ErrFailedToConvertLabelSelector      = errors.New("failed to convert label selector to selector")
	ErrFailedToGetGatewayByLabelSelector = errors.New("failed to get gateway by label selector")
	ErrFailedToAddOwnerReference         = errors.New("failed to add owner reference")
	ErrCantFindMatchingGateway           = errors.New("can't find matching Istio Gateway")
	ErrInvalidArgument                   = errors.New("invalid argument")
	ErrCantFindGatewayServersHost        = errors.New("can't find Istio Gateway servers hosts")
)
