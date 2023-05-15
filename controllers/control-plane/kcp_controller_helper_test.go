package control_plane_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	ErrNotFound                 = errors.New("resource not exists")
	ErrExpectedLabelNotReset    = errors.New("expected label not reset")
	ErrWatcherLabelMissing      = errors.New("watcher label missing")
	ErrWatcherAnnotationMissing = errors.New("watcher annotation missing")
)

func RegisterControlPlaneLifecycleForKyma(kyma *v1beta2.Kyma) {
	BeforeAll(func() {
		DeployModuleTemplates(ctx, controlPlaneClient, kyma, false, false, false)
		Eventually(CreateCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(controlPlaneClient, kyma).Should(Succeed())
	})

	AfterAll(func() {
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(controlPlaneClient, kyma).Should(Succeed())
		DeleteModuleTemplates(ctx, controlPlaneClient, kyma, false)
	})

	BeforeEach(func() {
		By("get latest kyma CR")
		Eventually(SyncKyma, Timeout, Interval).
			WithContext(ctx).WithArguments(controlPlaneClient, kyma).Should(Succeed())
	})
}

func UpdateRemoteModule(
	ctx context.Context,
	client client.Client,
	kyma *v1beta2.Kyma,
	remoteSyncNamespace string,
	modules []v1beta2.Module,
) func() error {
	return func() error {
		kyma, err := GetKyma(ctx, client, kyma.Name, remoteSyncNamespace)
		if err != nil {
			return err
		}
		kyma.Spec.Modules = modules
		return client.Update(ctx, kyma)
	}
}

func GetModuleTemplate(clnt client.Client, name, namespace string) (*v1beta2.ModuleTemplate, error) {
	moduleTemplateInCluster := &v1beta2.ModuleTemplate{}
	moduleTemplateInCluster.SetNamespace(namespace)
	moduleTemplateInCluster.SetName(name)
	err := clnt.Get(ctx, client.ObjectKeyFromObject(moduleTemplateInCluster), moduleTemplateInCluster)
	if err != nil {
		return nil, err
	}
	return moduleTemplateInCluster, nil
}

func KymaExists(clnt client.Client, name, namespace string) error {
	_, err := GetKyma(ctx, clnt, name, namespace)
	if k8serrors.IsNotFound(err) {
		return ErrNotFound
	}
	return nil
}

func ManifestExists(kyma *v1beta2.Kyma, module v1beta2.Module) error {
	_, err := GetManifest(ctx, controlPlaneClient, kyma, module)
	if k8serrors.IsNotFound(err) {
		return ErrNotFound
	}
	return nil
}

func ModuleTemplateExists(client client.Client, name, namespace string) error {
	_, err := GetModuleTemplate(client, name, namespace)
	if k8serrors.IsNotFound(err) {
		return ErrNotFound
	}
	return nil
}

func ModuleTemplatesExist(clnt client.Client, kyma *v1beta2.Kyma, remoteSyncNamespace string) func() error {
	return func() error {
		for _, module := range kyma.Spec.Modules {
			if err := ModuleTemplateExists(clnt, module.Name, remoteSyncNamespace); err != nil {
				return err
			}
		}

		return nil
	}
}

func WatcherLabelsAnnotationsExist(clnt client.Client, kyma *v1beta2.Kyma, remoteSyncNamespace string) error {
	remoteKyma, err := GetKyma(ctx, clnt, kyma.GetName(), remoteSyncNamespace)
	if err != nil {
		return err
	}
	if remoteKyma.Labels[v1beta2.WatchedByLabel] != v1beta2.OperatorName {
		return ErrWatcherLabelMissing
	}
	if remoteKyma.Annotations[v1beta2.OwnedByAnnotation] != fmt.Sprintf(v1beta2.OwnedByFormat,
		kyma.GetNamespace(), kyma.GetName()) {
		return ErrWatcherAnnotationMissing
	}
	return nil
}

func expectModuleTemplateSpecGetReset(
	clnt client.Client,
	moduleNamespace,
	moduleName,
	expectedValue string,
) error {
	moduleTemplate, err := GetModuleTemplate(clnt, moduleName, moduleNamespace)
	if err != nil {
		return err
	}
	initKey, found := moduleTemplate.Spec.Data.Object["spec"]
	if !found {
		return ErrExpectedLabelNotReset
	}
	value, found := initKey.(map[string]any)["initKey"]
	if !found {
		return ErrExpectedLabelNotReset
	}
	if value.(string) != expectedValue {
		return ErrExpectedLabelNotReset
	}
	return nil
}

func updateModuleTemplateSpec(clnt client.Client,
	moduleNamespace,
	moduleName,
	newValue string,
) error {
	moduleTemplate, err := GetModuleTemplate(clnt, moduleName, moduleNamespace)
	if err != nil {
		return err
	}
	moduleTemplate.Spec.Data.Object["spec"] = map[string]any{"initKey": newValue}
	return clnt.Update(ctx, moduleTemplate)
}
