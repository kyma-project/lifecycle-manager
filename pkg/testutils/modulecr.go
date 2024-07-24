package testutils

import (
	"context"
	"errors"
	"fmt"

	templatev1alpha1 "github.com/kyma-project/template-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/strings/slices"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

const (
	TestModuleCRName                   = "sample-yaml"
	TestModuleResourceNamespace        = "template-operator-system"
	ModuleResourceName                 = "template-operator-controller-manager"
	ModuleServiceAccountName           = "template-operator-controller-manager"
	ModuleManagedCRName                = "template-operator-managed-resource"
	ModuleDeploymentNameInNewerVersion = "template-operator-v2-controller-manager"
	ModuleDeploymentNameInOlderVersion = "template-operator-v1-controller-manager"
)

var (
	errSampleCRDeletionTimestampSet    = errors.New("sample CR has set DeletionTimeStamp")
	errSampleCRDeletionTimestampNotSet = errors.New("sample CR has not set DeletionTimeStamp")
	errFinalizerStillExists            = errors.New("finalizer still exists after purge timeout")
)

func ModuleCRExists(ctx context.Context, clnt client.Client, moduleCR *unstructured.Unstructured) error {
	err := clnt.Get(ctx, client.ObjectKey{
		Namespace: moduleCR.GetNamespace(),
		Name:      moduleCR.GetName(),
	}, moduleCR)
	return CRExists(moduleCR, err)
}

// NewTestModuleCR init one module cr used by template-operator.
func NewTestModuleCR(namespace string) *unstructured.Unstructured {
	return builder.NewModuleCRBuilder().
		WithName(TestModuleCRName).
		WithNamespace(namespace).Build()
}

func SampleCRNoDeletionTimeStampSet(ctx context.Context, name, namespace string, clnt client.Client) error {
	exists, err := DeletionTimeStampExists(ctx, shared.OperatorGroup, templatev1alpha1.GroupVersion.Version,
		string(templatev1alpha1.SampleKind), name, namespace, clnt)
	if err != nil {
		return err
	}

	if exists {
		return errSampleCRDeletionTimestampSet
	}
	return nil
}

func SampleCRDeletionTimeStampSet(ctx context.Context, name, namespace string, clnt client.Client) error {
	exists, err := DeletionTimeStampExists(ctx, shared.OperatorGroup, templatev1alpha1.GroupVersion.Version,
		string(templatev1alpha1.SampleKind), name, namespace, clnt)
	if err != nil {
		return err
	}

	if !exists {
		return errSampleCRDeletionTimestampNotSet
	}
	return nil
}

func AddFinalizerToModuleCR(ctx context.Context, clnt client.Client, moduleCR *unstructured.Unstructured,
	finalizer string,
) error {
	err := clnt.Get(ctx, client.ObjectKey{
		Namespace: moduleCR.GetNamespace(),
		Name:      moduleCR.GetName(),
	}, moduleCR)
	if err != nil {
		return fmt.Errorf("failed to get moduleCR %w", err)
	}

	finalizers := moduleCR.GetFinalizers()
	if finalizers == nil {
		finalizers = []string{}
	}
	moduleCR.SetFinalizers(append(finalizers, finalizer))

	if err = clnt.Update(ctx, moduleCR); err != nil {
		return fmt.Errorf("updating module CR %w", err)
	}

	return nil
}

func FinalizerIsRemoved(ctx context.Context, clnt client.Client, moduleCR *unstructured.Unstructured,
	finalizer string,
) error {
	err := clnt.Get(ctx, client.ObjectKey{
		Namespace: moduleCR.GetNamespace(),
		Name:      moduleCR.GetName(),
	}, moduleCR)

	if util.IsNotFound(err) {
		return nil
	}

	if slices.Contains(moduleCR.GetFinalizers(), finalizer) {
		return errFinalizerStillExists
	}

	return nil
}

func ModuleCRIsInExpectedState(ctx context.Context,
	clnt client.Client,
	moduleCR *unstructured.Unstructured,
	expectedState shared.State,
) bool {
	err := clnt.Get(ctx, client.ObjectKey{
		Namespace: moduleCR.GetNamespace(),
		Name:      moduleCR.GetName(),
	}, moduleCR)
	if err != nil {
		return false
	}

	state, _, err := unstructured.NestedString(moduleCR.Object, "status", "state")
	if err != nil {
		return false
	}
	return state == string(expectedState)
}
