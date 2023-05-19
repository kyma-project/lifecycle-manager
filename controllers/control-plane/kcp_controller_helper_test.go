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
	ErrExpectedLabelNotReset    = errors.New("expected label not reset")
	ErrWatcherLabelMissing      = errors.New("watcher label missing")
	ErrWatcherAnnotationMissing = errors.New("watcher annotation missing")
)

func registerControlPlaneLifecycleForKyma(kyma *v1beta2.Kyma) {
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

func updateRemoteModule(
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

func kymaExists(clnt client.Client, name, namespace string) error {
	_, err := GetKyma(ctx, clnt, name, namespace)
	if k8serrors.IsNotFound(err) {
		return ErrNotFound
	}
	return nil
}

func watcherLabelsAnnotationsExist(clnt client.Client, kyma *v1beta2.Kyma, remoteSyncNamespace string) error {
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
	moduleTemplate, err := GetModuleTemplate(ctx, clnt, moduleName, moduleNamespace)
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
	moduleTemplate, err := GetModuleTemplate(ctx, clnt, moduleName, moduleNamespace)
	if err != nil {
		return err
	}
	moduleTemplate.Spec.Data.Object["spec"] = map[string]any{"initKey": newValue}
	return clnt.Update(ctx, moduleTemplate)
}
