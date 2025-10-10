package resources_test

import (
	"net"
	"reflect"
	"strconv"
	"testing"

	apiappsv1 "k8s.io/api/apps/v1"
	apicorev1 "k8s.io/api/core/v1"
	apinetworkv1 "k8s.io/api/networking/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	skrwebhookresources "github.com/kyma-project/lifecycle-manager/internal/service/watcher/resources"
)

func toUnstructured(obj interface{}) *unstructured.Unstructured {
	m, _ := machineryruntime.DefaultUnstructuredConverter.ToUnstructured(obj)
	return &unstructured.Unstructured{Object: m}
}

func TestResourceConfigurator_ConfigureDeployment_SetsGODEBUG(t *testing.T) {
	expectedEnvValue := "dummyvalue"
	expectedEnvName := "GODEBUG"
	t.Setenv(expectedEnvName, expectedEnvValue)

	testDeploy := toUnstructured(&apiappsv1.Deployment{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:   "dbg-deploy",
			Labels: map[string]string{"dbg": "true"},
		},
		Spec: apiappsv1.DeploymentSpec{
			Template: apicorev1.PodTemplateSpec{
				ObjectMeta: apimetav1.ObjectMeta{
					Labels: map[string]string{"app": "dbg"},
				},
				Spec: apicorev1.PodSpec{
					Containers: []apicorev1.Container{
						{
							Name:  "main",
							Image: "old-image",
						},
					},
				},
			},
		},
	})

	configurator := skrwebhookresources.NewResourceConfigurator(
		"",
		"",
		"100m",
		"128Mi",
		skrwebhookresources.KCPAddr{Hostname: "dbg-host", Port: 4242},
	)
	configurator.SetSecretResVer("v1")

	got, err := configurator.ConfigureDeployment(testDeploy)
	if err != nil {
		t.Fatalf("ConfigureDeployment() returned error: %v", err)
	}

	found := false
	for _, env := range got.Spec.Template.Spec.Containers[0].Env {
		if env.Name == expectedEnvName {
			if env.Value != expectedEnvValue {
				t.Fatalf("GODEBUG value = %q, want %q", env.Value, expectedEnvValue)
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("GODEBUG env not found in container env: %+v", got.Spec.Template.Spec.Containers[0].Env)
	}
}

//nolint:gocognit // test case is complex
func TestResourceConfigurator_ConfigureDeployment(t *testing.T) {
	kcpAddr := skrwebhookresources.KCPAddr{Hostname: "test-host", Port: 8080}
	cpuLimit := "100m"
	memLimit := "128Mi"
	secretResVer := "v1"
	watcherImage := "test-image:latest"

	tests := []struct {
		name      string
		fields    fields
		obj       *unstructured.Unstructured
		wantImage string
		wantEnv   string
		wantErr   bool
	}{
		{
			name: "sets image, env, resource limits, and PodRestartLabelKey",
			fields: fields{
				kcpAddress:      kcpAddr,
				cpuResLimit:     cpuLimit,
				memResLimit:     memLimit,
				secretResVer:    secretResVer,
				skrWatcherImage: watcherImage,
			},
			obj: toUnstructured(&apiappsv1.Deployment{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:   "test-deploy",
					Labels: map[string]string{"foo": "bar"},
				},
				Spec: apiappsv1.DeploymentSpec{
					Template: apicorev1.PodTemplateSpec{
						ObjectMeta: apimetav1.ObjectMeta{
							Labels: map[string]string{"app": "test"},
						},
						Spec: apicorev1.PodSpec{
							Containers: []apicorev1.Container{
								{
									Name:  "main",
									Image: "old-image",
									Env: []apicorev1.EnvVar{
										{Name: "KCP_ADDR", Value: ""},
									},
								},
							},
						},
					},
				},
			}),
			wantImage: watcherImage,
			wantEnv:   net.JoinHostPort(kcpAddr.Hostname, strconv.Itoa(int(kcpAddr.Port))),
			wantErr:   false,
		},
		{
			name: "error on empty pod template labels",
			fields: fields{
				kcpAddress:      kcpAddr,
				cpuResLimit:     cpuLimit,
				memResLimit:     memLimit,
				skrWatcherImage: watcherImage,
				secretResVer:    secretResVer,
			},
			obj: toUnstructured(&apiappsv1.Deployment{
				ObjectMeta: apimetav1.ObjectMeta{Name: "test"},
				Spec: apiappsv1.DeploymentSpec{
					Template: apicorev1.PodTemplateSpec{
						ObjectMeta: apimetav1.ObjectMeta{Labels: map[string]string{}},
						Spec:       apicorev1.PodSpec{Containers: []apicorev1.Container{{Name: "main"}}},
					},
				},
			}),
			wantErr: true,
		},
		{
			name: "error on empty containers",
			fields: fields{
				kcpAddress:      kcpAddr,
				cpuResLimit:     cpuLimit,
				memResLimit:     memLimit,
				skrWatcherImage: watcherImage,
				secretResVer:    secretResVer,
			},
			obj: toUnstructured(&apiappsv1.Deployment{
				ObjectMeta: apimetav1.ObjectMeta{Name: "test"},
				Spec: apiappsv1.DeploymentSpec{
					Template: apicorev1.PodTemplateSpec{
						ObjectMeta: apimetav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
						Spec:       apicorev1.PodSpec{Containers: []apicorev1.Container{}},
					},
				},
			}),
			wantErr: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			configurator := skrwebhookresources.NewResourceConfigurator(
				testCase.fields.remoteNs,
				testCase.fields.skrWatcherImage,
				testCase.fields.cpuResLimit,
				testCase.fields.memResLimit,
				testCase.fields.kcpAddress,
			)
			configurator.SetSecretResVer(testCase.fields.secretResVer)
			got, err := configurator.ConfigureDeployment(testCase.obj)
			if (err != nil) != testCase.wantErr {
				t.Errorf("ConfigureDeployment() error = %v, wantErr %v", err, testCase.wantErr)
				return
			}
			//nolint:nestif // test case
			if err == nil {
				if got.Spec.Template.Spec.Containers[0].Image != testCase.wantImage {
					t.Errorf("Image = %v, want %v", got.Spec.Template.Spec.Containers[0].Image, testCase.wantImage)
				}
				found := false
				for _, env := range got.Spec.Template.Spec.Containers[0].Env {
					if env.Name == "KCP_ADDR" && env.Value == testCase.wantEnv {
						found = true
					}
				}
				if !found {
					t.Errorf("KCP_ADDR env not set correctly, want %v", testCase.wantEnv)
				}
				if got.Spec.Template.Labels[skrwebhookresources.PodRestartLabelKey] != secretResVer {
					t.Errorf("PodRestartLabelKey = %v, want %v",
						got.Spec.Template.Labels[skrwebhookresources.PodRestartLabelKey],
						secretResVer)
				}
				if got.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu().String() != cpuLimit {
					t.Errorf("CPU limit = %v, want %v",
						got.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu().String(), cpuLimit)
				}
				if got.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().String() != memLimit {
					t.Errorf("Memory limit = %v, want %v",
						got.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().String(), memLimit)
				}
			}
		})
	}
}

func TestResourceConfigurator_ConfigureNetworkPolicies(t *testing.T) {
	kcpAddr := skrwebhookresources.KCPAddr{Hostname: "test-host", Port: 1234}
	protocol := apicorev1.ProtocolTCP
	port := intstr.FromInt32(1234)

	tests := []struct {
		name    string
		fields  fields
		obj     *unstructured.Unstructured
		want    *apinetworkv1.NetworkPolicy
		wantErr bool
	}{
		{
			name:   "updates egress for ApiServerNetworkPolicyName",
			fields: fields{kcpAddress: kcpAddr},
			obj: toUnstructured(&apinetworkv1.NetworkPolicy{
				ObjectMeta: apimetav1.ObjectMeta{Name: skrwebhookresources.ApiServerNetworkPolicyName},
			}),
			want: &apinetworkv1.NetworkPolicy{
				ObjectMeta: apimetav1.ObjectMeta{Name: skrwebhookresources.ApiServerNetworkPolicyName},
				Spec: apinetworkv1.NetworkPolicySpec{
					Egress: []apinetworkv1.NetworkPolicyEgressRule{
						{
							Ports: []apinetworkv1.NetworkPolicyPort{
								{Protocol: &protocol, Port: &port},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:   "no egress update for other policy",
			fields: fields{kcpAddress: kcpAddr},
			obj: toUnstructured(&apinetworkv1.NetworkPolicy{
				ObjectMeta: apimetav1.ObjectMeta{Name: "other-policy"},
			}),
			want: &apinetworkv1.NetworkPolicy{
				ObjectMeta: apimetav1.ObjectMeta{Name: "other-policy"},
			},
			wantErr: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			configurator := skrwebhookresources.NewResourceConfigurator(
				testCase.fields.remoteNs,
				testCase.fields.skrWatcherImage,
				testCase.fields.cpuResLimit,
				testCase.fields.memResLimit,
				testCase.fields.kcpAddress)
			got, err := configurator.ConfigureNetworkPolicies(testCase.obj)
			if (err != nil) != testCase.wantErr {
				t.Errorf("ConfigureNetworkPolicies() error = %v, wantErr %v", err, testCase.wantErr)
				return
			}
			if !testCase.wantErr && !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("ConfigureNetworkPolicies() got = %v, want %v", got, testCase.want)
			}
		})
	}
}

type fields struct {
	remoteNs        string
	skrWatcherImage string
	secretResVer    string
	kcpAddress      skrwebhookresources.KCPAddr
	cpuResLimit     string
	memResLimit     string
}
