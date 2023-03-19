package v1_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	. "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	testv1 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2/test/v1"

	"golang.org/x/time/rate"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

//nolint:gochecknoglobals
var (
	// this is a unique base testing directory that will be used within a given run
	// it is expected to be removed externally (e.g. by testing.T) to cleanup leftovers
	// (e.g. cached manifests).
	testDir        string
	testSamplesDir = filepath.Join("..", "..", "..", "..", "..", "pkg", "test_samples")

	env        *envtest.Environment
	cfg        *rest.Config
	testClient client.Client

	// this namespace determines where the CustomResource instances will be created. It is purposefully static,
	// not because it would not be possible to make it random, but because the CRs should be able to install
	// and even create other namespaces than this one dynamically, and we will need to test this.
	customResourceNamespace = &v1.Namespace{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Namespace"},
		ObjectMeta: metav1.ObjectMeta{Name: "kyma-system"},
	}
)

func TestAPIs(t *testing.T) {
	testDir = t.TempDir()
	t.Parallel()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Declarative V2 Test Suite")
}

var _ = Describe(
	"Test Declarative Reconciliation", func() {
		var runID string
		var ctx context.Context
		var cancel context.CancelFunc
		BeforeEach(func() { runID = fmt.Sprintf("run-%s", rand.String(4)) })
		BeforeEach(func() { ctx, cancel = context.WithCancel(context.TODO()) })
		AfterEach(func() { cancel() })

		tableTest := func(
			spec testv1.TestAPISpec,
			source *CustomSpecFns,
			opts []Option,
			testCase func(ctx context.Context, key client.ObjectKey, source *CustomSpecFns),
		) {
			StartDeclarativeReconcilerForRun(ctx, runID, append(opts, WithSpecResolver(source))...)

			obj := &testv1.TestAPI{Spec: spec}
			obj.SetLabels(labels.Set{testRunLabel: runID})
			// this namespace is different form the test-run and path as we may need to test namespace creation
			obj.SetNamespace(customResourceNamespace.Name)
			obj.SetName(runID)
			Expect(testClient.Create(ctx, obj)).To(Succeed())
			key := client.ObjectKeyFromObject(obj)

			EventuallyDeclarativeStatusShould(
				ctx, key,
				BeInState(StateReady),
				HaveConditionWithStatus(ConditionTypeResources, metav1.ConditionTrue),
				HaveConditionWithStatus(ConditionTypeInstallation, metav1.ConditionTrue),
			)

			Expect(testClient.Get(ctx, key, obj)).To(Succeed())
			Expect(obj.GetStatus()).To(HaveAllSyncedResourcesExistingInCluster(ctx))

			if testCase != nil {
				testCase(ctx, key, source)
			}

			Expect(testClient.Delete(ctx, obj)).To(Succeed())

			EventuallyDeclarativeShouldBeUninstalled(ctx, obj)
		}

		DescribeTable(
			fmt.Sprintf("Starting Controller and Testing Declarative Reconciler (Run %s)", runID),
			tableTest,
			Entry(
				"Create simple chart from CR without modifications and become ready",
				testv1.TestAPISpec{ManifestName: "simple-helm"},
				DefaultSpec(filepath.Join(testSamplesDir, "module-chart"), map[string]any{}, RenderModeHelm),
				[]Option{},
				func(ctx context.Context, key client.ObjectKey, source *CustomSpecFns) {
					EventuallyDeclarativeStatusShould(
						ctx, key,
						HaveConditionWithStatus(ConditionTypeHelmCRDs, metav1.ConditionTrue),
					)
				},
			),
			Entry(
				"Create simple chart with a different Control Plane and Runtime Client",
				testv1.TestAPISpec{ManifestName: "custom-client"},
				DefaultSpec(filepath.Join(testSamplesDir, "module-chart"), map[string]any{}, RenderModeHelm),
				[]Option{WithRemoteTargetCluster(
					func(context.Context, Object) (*ClusterInfo, error) {
						return &ClusterInfo{
							Config: cfg,
						}, nil
					},
				)},
				nil,
			),
			Entry(
				"Create simple chart from CR from TGZ with CRDs and become ready",
				testv1.TestAPISpec{ManifestName: "tgz-with-crds"},
				DefaultSpec(
					filepath.Join(testSamplesDir, "oci", "helm_chart_with_crds.tgz"), map[string]any{},
					RenderModeHelm,
				),
				[]Option{},
				func(ctx context.Context, key client.ObjectKey, source *CustomSpecFns) {
					EventuallyDeclarativeStatusShould(
						ctx, key,
						HaveConditionWithStatus(ConditionTypeHelmCRDs, metav1.ConditionTrue),
					)
				},
			),
			Entry(
				"Create simple kustomization",
				testv1.TestAPISpec{ManifestName: "simple-kustomization"},
				DefaultSpec(
					filepath.Join(testSamplesDir, "kustomize"), map[string]any{"AddManagedbyLabel": true},
					RenderModeKustomize,
				),
				[]Option{},
				nil,
			),
			Entry(
				"Create simple Raw manifest",
				testv1.TestAPISpec{ManifestName: "simple-raw"},
				DefaultSpec(
					filepath.Join(testSamplesDir, "raw-manifest.yaml"), map[string]any{}, RenderModeRaw,
				),
				[]Option{},
				nil,
			),
			Entry(
				"Recreation of resources after external delete",
				testv1.TestAPISpec{ManifestName: "recreation-of-resources"},
				DefaultSpec(
					filepath.Join(testSamplesDir, "module-chart"), map[string]any{}, RenderModeHelm,
				),
				[]Option{},
				func(ctx context.Context, key client.ObjectKey, source *CustomSpecFns) {
					obj := &testv1.TestAPI{}
					Expect(testClient.Get(ctx, key, obj)).To(Succeed())
					Eventually(removeResourcesInCluster, standardTimeout, standardInterval).
						WithContext(ctx).
						WithArguments(obj).
						Should(Succeed())
					Eventually(expectResourceRecreated, standardTimeout, standardInterval).
						WithContext(ctx).
						WithArguments(obj).
						Should(Succeed())
				},
			),
			Entry(
				"Change values.yaml input and expect new Resource to be synced",
				testv1.TestAPISpec{ManifestName: "helm-values-change"},
				DefaultSpec(
					filepath.Join(testSamplesDir, "module-chart"),
					map[string]any{},
					RenderModeHelm,
				),
				[]Option{},
				func(ctx context.Context, key client.ObjectKey, source *CustomSpecFns) {
					obj := &testv1.TestAPI{}
					Expect(testClient.Get(ctx, key, obj)).To(Succeed())
					oldAmount := len(obj.GetStatus().Synced)
					EventuallyDeclarativeStatusShould(
						ctx, key,
						HaveSyncedResources(oldAmount),
						BeInState(StateReady),
					)
					source.ValuesFn = func(_ context.Context, _ Object) any {
						return map[string]any{"autoscaling": map[string]any{"enabled": true}}
					}
					EventuallyDeclarativeStatusShould(
						ctx, key,
						HaveSyncedResources(oldAmount+1),
						BeInState(StateReady),
					)
					source.ValuesFn = func(_ context.Context, _ Object) any {
						return map[string]any{"autoscaling": map[string]any{"enabled": false}}
					}
					EventuallyDeclarativeStatusShould(
						ctx, key,
						HaveSyncedResources(oldAmount),
						BeInState(StateReady),
					)
				},
			),
			Entry(
				"Custom Hooks",
				testv1.TestAPISpec{ManifestName: "custom-pre-post-hooks"},
				DefaultSpec(
					filepath.Join(testSamplesDir, "module-chart"),
					map[string]any{},
					RenderModeHelm,
				),
				[]Option{
					WithPostRun{func(ctx context.Context, skr Client, kcp client.Client, obj Object) error {
						hookID := fmt.Sprintf("%s-%s", obj.GetName(), "test-hooks")
						configMap := &v1.ConfigMap{}
						configMap.SetName(hookID)
						configMap.SetNamespace(customResourceNamespace.GetName())
						if err := skr.Create(ctx, configMap); err != nil && !errors.IsAlreadyExists(err) {
							return err
						}
						if added := controllerutil.AddFinalizer(obj, hookID); added {
							if err := kcp.Update(ctx, obj); err != nil {
								return err
							}
							obj.SetManagedFields(nil)
						}
						return nil
					}},
					WithPreDelete{func(ctx context.Context, skr Client, kcp client.Client, obj Object) error {
						hookID := fmt.Sprintf("%s-%s", obj.GetName(), "test-hooks")
						configMap := &v1.ConfigMap{}
						configMap.SetName(hookID)
						configMap.SetNamespace(customResourceNamespace.GetName())
						if err := skr.Delete(ctx, configMap); err != nil && !errors.IsNotFound(err) {
							return err
						}
						if removed := controllerutil.RemoveFinalizer(obj, hookID); removed {
							if err := kcp.Update(ctx, obj); err != nil {
								return err
							}
							obj.SetManagedFields(nil)
						}
						return nil
					}},
				},
				func(ctx context.Context, key client.ObjectKey, source *CustomSpecFns) {
					hookKey := client.ObjectKey{
						Name:      fmt.Sprintf("%s-%s", key.Name, "test-hooks"),
						Namespace: key.Namespace,
					}
					Expect(testClient.Get(ctx, hookKey, &v1.ConfigMap{})).Should(Succeed())
				},
			),
		)
	},
)

// StartDeclarativeReconcilerForRun starts the declarative reconciler based on a runID.
func StartDeclarativeReconcilerForRun(
	ctx context.Context,
	runID string,
	options ...Option,
) {
	var (
		namespace  = fmt.Sprintf("%s-%s", "test", runID)
		finalizer  = fmt.Sprintf("%s-%s", FinalizerDefault, runID)
		mgr        ctrl.Manager
		reconciler reconcile.Reconciler
		err        error
	)

	mgr, err = ctrl.NewManager(
		cfg, ctrl.Options{
			// these bind addreses cause conflicts when run concurrently so we disable them
			HealthProbeBindAddress: "0",
			MetricsBindAddress:     "0",
			Scheme:                 scheme.Scheme,
		},
	)
	Expect(err).ToNot(HaveOccurred())

	reconciler = NewFromManager(
		mgr, &testv1.TestAPI{},
		append(
			options,
			WithNamespace(namespace, true),
			WithFinalizer(finalizer),
			// we overwride the manifest cache directory with the test run directory so its automatically cleaned up
			// we ensure uniqueness implicitly, as runID is used to randomize the ManifestName in SpecResolver
			WithManifestCache(filepath.Join(testDir, "declarative-test-cache")),
			// we have to use a custom ready check that only checks for existence of an object since the default
			// readiness check will not work without dedicated control loops in envtest. E.g. by default
			// deployments are not started or set to ready. However we can check if the resource was created by
			// the reconciler.
			WithClientCacheKey(),
			WithCustomReadyCheck(NewExistsReadyCheck()),
			WithCustomResourceLabels(labels.Set{testRunLabel: runID}),
			WithPeriodicConsistencyCheck(2*time.Second),
		)...,
	)

	// in case there is any leak of CRs from another test run, but this is most likely never necessary
	testWatchPredicate, err := predicate.LabelSelectorPredicate(
		metav1.LabelSelector{MatchLabels: labels.Set{testRunLabel: runID}},
	)
	Expect(err).ToNot(HaveOccurred())

	Expect(
		ctrl.NewControllerManagedBy(mgr).WithEventFilter(testWatchPredicate).
			WithOptions(
				controller.Options{RateLimiter: workqueue.NewMaxOfRateLimiter(
					&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(30), 200)},
				)},
			).
			For(&testv1.TestAPI{}).Complete(reconciler),
	).To(Succeed())
	go func() {
		Expect(mgr.Start(ctx)).To(Succeed(), "failed to run manager")
	}()
}

func StatusOnCluster(g Gomega, ctx context.Context, key client.ObjectKey) Status { //nolint:revive
	obj := &testv1.TestAPI{}
	g.Expect(testClient.Get(ctx, key, obj)).To(Succeed())
	return obj.GetStatus()
}

func removeResourcesInCluster(ctx context.Context, obj *testv1.TestAPI) error {
	synced := obj.GetStatus().Synced
	for i := range synced {
		unstruct := synced[i].ToUnstructured()
		ExpectWithOffset(1, testClient.Delete(ctx, unstruct)).To(Succeed())
	}
	return nil
}

func expectResourceRecreated(ctx context.Context, obj *testv1.TestAPI) error {
	if err := testClient.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
		return err
	}

	synced := obj.GetStatus().Synced

	for i := range synced {
		unstruct := synced[i].ToUnstructured()
		if err := testClient.Get(ctx, client.ObjectKeyFromObject(unstruct), unstruct); err != nil {
			return err
		}
	}

	return nil
}

func WithClientCacheKey() WithClientCacheKeyOption {
	cacheKey := func(ctx context.Context, resource Object) (any, bool) {
		return client.ObjectKeyFromObject(resource), true
	}
	return WithClientCacheKeyOption{ClientCacheKeyFn: cacheKey}
}
