package resources

import (
	"errors"
	"fmt"
	"net"
	"strconv"

	apiappsv1 "k8s.io/api/apps/v1"
	apicorev1 "k8s.io/api/core/v1"
	apinetworkv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/util/collections"
)

type ResourceConfigurator struct {
	remoteNs                 string
	skrWatcherImage          string
	secretResVer             string
	kcpAddress               KCPAddr
	cpuResLimit, memResLimit string
	objectHandlers           map[string]ObjectHandler
}

type KCPAddr struct {
	Hostname string
	Port     uint32
}

var (
	errExpectedNonEmptyPodContainers    = errors.New("expected non empty pod containers")
	errPodTemplateMustContainAtLeastOne = errors.New("pod template labels must contain " +
		"at least the deployment selector label")
	errConvertUnstruct         = errors.New("failed to convert deployment to unstructured")
	errNoSecretResourceVersion = errors.New("secret resource version must not be empty")
)

const (
	PodRestartLabelKey             = shared.OperatorGroup + shared.Separator + "pod-restart-trigger"
	kcpAddressEnvName              = "KCP_ADDR"
	ApiServerNetworkPolicyName     = "kyma-project.io--watcher-to-apiserver"
	SeedToWatcherNetworkPolicyName = "kyma-project.io--seed-to-watcher"
	WatcherToDNSNetworkPolicyName  = "kyma-project.io--watcher-to-dns"
	MetricsToWatcherPolicyName     = "kyma-project.io--metrics-to-watcher"
)

func NewResourceConfigurator(remoteNs, skrWatcherImage,
	cpuResLimit, memResLimit string, kcpAddress KCPAddr,
) *ResourceConfigurator {
	configurator := &ResourceConfigurator{
		remoteNs:        remoteNs,
		skrWatcherImage: skrWatcherImage,
		kcpAddress:      kcpAddress,
		cpuResLimit:     cpuResLimit,
		memResLimit:     memResLimit,
	}

	configurator.objectHandlers = map[string]ObjectHandler{
		apiappsv1.SchemeGroupVersion.String() + "/Deployment": func(rc *ResourceConfigurator,
			obj *unstructured.Unstructured,
		) (client.Object, error) {
			return rc.ConfigureDeployment(obj)
		},
		apinetworkv1.SchemeGroupVersion.String() + "/NetworkPolicy": func(rc *ResourceConfigurator,
			obj *unstructured.Unstructured,
		) (client.Object, error) {
			return rc.ConfigureNetworkPolicies(obj)
		},
	}
	return configurator
}

func (rc *ResourceConfigurator) SetSecretResVer(secretResVer string) {
	rc.secretResVer = secretResVer
}

type ObjectHandler func(rc *ResourceConfigurator, obj *unstructured.Unstructured) (client.Object, error)

func (rc *ResourceConfigurator) ConfigureUnstructuredObject(object *unstructured.Unstructured) (client.Object, error) {
	key := object.GetAPIVersion() + "/" + object.GetKind()
	if handler, ok := rc.objectHandlers[key]; ok {
		return handler(rc, object)
	}
	return object.DeepCopy(), nil
}

func (rc *ResourceConfigurator) ConfigureDeployment(obj *unstructured.Unstructured,
) (*apiappsv1.Deployment, error) {
	deployment := &apiappsv1.Deployment{}
	if err := machineryruntime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, deployment); err != nil {
		return nil, fmt.Errorf("%w: %w", errConvertUnstruct, err)
	}
	if len(deployment.Spec.Template.Labels) == 0 {
		return nil, errPodTemplateMustContainAtLeastOne
	}
	if len(deployment.Spec.Template.Spec.Containers) == 0 {
		return nil, errExpectedNonEmptyPodContainers
	}
	if rc.secretResVer == "" {
		return nil, fmt.Errorf("secret resource version must not be empty: %w", errNoSecretResourceVersion)
	}
	deployment.Spec.Template.Labels[PodRestartLabelKey] = rc.secretResVer

	serverContainer := deployment.Spec.Template.Spec.Containers[0]
	serverContainer.Image = rc.skrWatcherImage

	for i := range len(serverContainer.Env) {
		if serverContainer.Env[i].Name == kcpAddressEnvName {
			serverContainer.Env[i].Value = net.JoinHostPort(rc.kcpAddress.Hostname,
				strconv.Itoa(int(rc.kcpAddress.Port)))
		}
	}

	cpuResQty, err := resource.ParseQuantity(rc.cpuResLimit)
	if err != nil {
		return nil, fmt.Errorf("error parsing CPU resource limit: %w", err)
	}
	memResQty, err := resource.ParseQuantity(rc.memResLimit)
	if err != nil {
		return nil, fmt.Errorf("error parsing memory resource limit: %w", err)
	}
	serverContainer.Resources.Limits = map[apicorev1.ResourceName]resource.Quantity{
		apicorev1.ResourceCPU:    cpuResQty,
		apicorev1.ResourceMemory: memResQty,
	}
	deployment.Spec.Template.Spec.Containers[0] = serverContainer

	deployment.SetLabels(collections.MergeMapsSilent(deployment.GetLabels(), map[string]string{
		shared.ManagedBy: shared.ManagedByLabelValue,
	}))

	return deployment, nil
}

func (rc *ResourceConfigurator) ConfigureNetworkPolicies(obj *unstructured.Unstructured) (*apinetworkv1.NetworkPolicy,
	error,
) {
	networkPolicy := &apinetworkv1.NetworkPolicy{}
	if err := machineryruntime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, networkPolicy); err != nil {
		return nil, fmt.Errorf("%w: %w", errConvertUnstruct, err)
	}

	if networkPolicy.GetObjectMeta().GetName() == ApiServerNetworkPolicyName {
		kcpPortInt := intstr.FromInt32(
			int32(rc.kcpAddress.Port), //nolint:gosec // G115: this is not a security sensitive code, just a port number
		)
		networkProtocol := apicorev1.ProtocolTCP

		egressRule := []apinetworkv1.NetworkPolicyEgressRule{
			{
				Ports: []apinetworkv1.NetworkPolicyPort{
					{
						Protocol: &networkProtocol,
						Port:     &kcpPortInt,
					},
				},
			},
		}

		networkPolicy.Spec.Egress = egressRule
	}

	return networkPolicy, nil
}
