package testutils

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
	"github.com/onsi/ginkgo/v2/dsl/core"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/strings/slices"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const TestModuleCRName = "sample-yaml"

var (
	ErrFinalizerNotFound    = errors.New("finalizer does not exist before purge timeout")
	ErrFinalizerStillExists = errors.New("finalizer still exists after purge timeout")
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

func AddFinalizerToModuleCR(ctx context.Context, clnt client.Client, moduleCR *unstructured.Unstructured,
	finalizer string,
) error {
	err := clnt.Get(ctx, client.ObjectKey{
		Namespace: moduleCR.GetNamespace(),
		Name:      moduleCR.GetName(),
	}, moduleCR)

	if err = CRExists(moduleCR, err); err != nil {
		return err
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

func FinalizerIsRemovedAfterTimeout(ctx context.Context, clnt client.Client, moduleCR *unstructured.Unstructured,
	finalizer string, deletionTime time.Time,
) error {
	err := clnt.Get(ctx, client.ObjectKey{
		Namespace: moduleCR.GetNamespace(),
		Name:      moduleCR.GetName(),
	}, moduleCR)

	crExistError := CRExists(moduleCR, err)

	if time.Now().Before(deletionTime) {
		core.GinkgoWriter.Println("BEFORE:", crExistError)
		if crExistError != nil {
			return fmt.Errorf("module cr does not exist %w", crExistError)
		}

		if !slices.Contains(moduleCR.GetFinalizers(), finalizer) {
			return ErrFinalizerNotFound
		}
	} else {
		core.GinkgoWriter.Println("AFTER:", crExistError)
		if util.IsNotFound(err) {
			return nil
		}

		if slices.Contains(moduleCR.GetFinalizers(), finalizer) {
			core.GinkgoWriter.Println("AFTER: FINALIZER NOT EXISTS")
			return ErrFinalizerStillExists
		}
	}

	return nil
}
