package declarative_test

import (
	"context"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/time/rate"
	apicorev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/rand"
	k8sclientscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlruntime "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
	"github.com/kyma-project/lifecycle-manager/tests/integration"
	declarativetest "github.com/kyma-project/lifecycle-manager/tests/integration/declarative"

	. "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	// this is a unique base testing directory that will be used within a given run
	// it is expected to be removed externally (e.g. by testing.T) to cleanup leftovers
	// (e.g. cached manifests).
	testDir        string
	testSamplesDir = filepath.Join(integration.GetProjectRoot(), "pkg", "test_samples")
	testAPICRD     *apiextensionsv1.CustomResourceDefinition
	// this namespace determines where the CustomResource instances will be created. It is purposefully static,
	// not because it would not be possible to make it random, but because the CRs should be able to install
	// and even create other namespaces than this one dynamically, and we will need to test this.
	customResourceNamespace = &apicorev1.Namespace{
		TypeMeta:   apimetav1.TypeMeta{APIVersion: "v1", Kind: "Namespace"},
		ObjectMeta: apimetav1.ObjectMeta{Name: "kyma-system"},
	}
	ErrOldResourcesStillDeployed = errors.New("old resources still exist in the cluster")
	ErrOldResourcesStillInSynced = errors.New("old resources still exist in the status.synced")
)

func TestAPIs(t *testing.T) {
	testDir = t.TempDir()
	t.Parallel()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Declarative V2 Test Suite")
}

var _ = Describe(
	"Test Declarative Reconciliation", Ordered, func() {
		var runID string
		var ctx context.Context
		var cancel context.CancelFunc
		var env *envtest.Environment
		var cfg *rest.Config
		var testClient client.Client
		BeforeAll(func() {
			runID = fmt.Sprintf("run-%s", rand.String(4))
			env, cfg = StartEnv()
			testClient = GetTestClient(cfg)
			ctx, cancel = context.WithCancel(context.TODO())
		})
		AfterAll(func() {
			cancel()
			Expect(env.Stop()).To(Succeed())
		})
		const ocirefSynced = "sha256:synced"

		tableTest := func(
			spec declarativetest.TestAPISpec,
			source *CustomSpecFns,
			opts []Option,
			testCase func(ctx context.Context, key client.ObjectKey, source *CustomSpecFns),
		) {
			StartDeclarativeReconcilerForRun(ctx, runID, cfg, append(opts, WithSpecResolver(source))...)
			obj := &declarativetest.TestAPI{Spec: spec}
			obj.SetLabels(k8slabels.Set{testRunLabel: runID})
			// this namespace is different form the test-run and path as we may need to test namespace creation
			obj.SetNamespace(customResourceNamespace.Name)
			obj.SetName(runID)
			Expect(testClient.Create(ctx, obj)).To(Succeed())
			key := client.ObjectKeyFromObject(obj)

			EventuallyDeclarativeStatusShould(
				ctx, key, testClient,
				BeInState(shared.StateReady),
				HaveConditionWithStatus(ConditionTypeResources, apimetav1.ConditionTrue),
				HaveConditionWithStatus(ConditionTypeInstallation, apimetav1.ConditionTrue),
			)

			Expect(testClient.Get(ctx, key, obj)).To(Succeed())
			Expect(obj.GetStatus()).To(HaveAllSyncedResourcesExistingInCluster(ctx, testClient))

			if testCase != nil {
				testCase(ctx, key, source)
			}

			Expect(testClient.Delete(ctx, obj)).To(Succeed())

			EventuallyDeclarativeShouldBeUninstalled(ctx, obj, testClient)
		}

		DescribeTable(
			fmt.Sprintf("Starting Controller and Testing Declarative Reconciler (Run %s)", runID),
			tableTest,
			Entry(
				"Create simple raw manifest with a different Control Plane and Runtime Client",
				declarativetest.TestAPISpec{ManifestName: "custom-client"},
				DefaultSpec(filepath.Join(testSamplesDir, "raw-manifest.yaml"), ocirefSynced, RenderModeRaw),
				[]Option{
					WithRemoteTargetCluster(
						func(context.Context, Object) (*ClusterInfo, error) {
							return &ClusterInfo{
								Config: cfg,
							}, nil
						},
					),
				},
				nil,
			),
			Entry(
				"Create simple Raw manifest",
				declarativetest.TestAPISpec{ManifestName: "simple-raw"},
				DefaultSpec(filepath.Join(testSamplesDir, "raw-manifest.yaml"), ocirefSynced, RenderModeRaw),
				[]Option{},
				nil,
			),
		)
	},
)

var _ = Describe("Test Manifest Reconciliation for module deletion", Ordered, func() {
	var ctx context.Context
	var cancel context.CancelFunc
	var reconciler *Reconciler
	var env *envtest.Environment
	var cfg *rest.Config
	var testClient client.Client
	const ocirefSynced = "sha256:synced"

	runID := fmt.Sprintf("run-%s", rand.String(4))
	obj := &declarativetest.TestAPI{Spec: declarativetest.TestAPISpec{ManifestName: "deletion-manifest"}}
	obj.SetLabels(k8slabels.Set{testRunLabel: runID})
	obj.SetNamespace(customResourceNamespace.Name)
	obj.SetName(runID)

	key := client.ObjectKeyFromObject(obj)

	opts := []Option{
		WithRemoteTargetCluster(
			func(context.Context, Object) (*ClusterInfo, error) {
				return &ClusterInfo{
					Config: cfg,
				}, nil
			},
		),
	}
	source := WithSpecResolver(DefaultSpec(filepath.Join(testSamplesDir, "raw-manifest.yaml"), ocirefSynced,
		RenderModeRaw))
	oldDeployedResources, err := internal.ParseManifestToObjects(path.Join(testSamplesDir, "raw-manifest.yaml"))
	Expect(err).NotTo(HaveOccurred())

	BeforeAll(func() {
		env, cfg = StartEnv()
		testClient = GetTestClient(cfg)
		ctx, cancel = context.WithCancel(context.TODO())
		reconciler = StartDeclarativeReconcilerForRun(ctx, runID, cfg, append(opts, WithSpecResolver(source))...)
	})

	It("Should create manifest resources", func() {
		Expect(testClient.Create(ctx, obj)).To(Succeed())

		EventuallyDeclarativeStatusShould(
			ctx, key, testClient,
			BeInState(shared.StateReady),
			HaveConditionWithStatus(ConditionTypeResources, apimetav1.ConditionTrue),
			HaveConditionWithStatus(ConditionTypeInstallation, apimetav1.ConditionTrue),
		)

		Expect(testClient.Get(ctx, key, obj)).To(Succeed())
		Expect(obj.GetStatus()).To(HaveAllSyncedResourcesExistingInCluster(ctx, testClient))
	})

	It("Should remove deployed module resources after its deletion", func() {
		source := WithSpecResolver(DefaultSpec(filepath.Join(testSamplesDir, "empty-file.yaml"),
			"", RenderModeRaw))
		reconciler.SpecResolver = source
		Eventually(validateOldResourcesNotLongerDeployed, Timeout, Interval).
			WithContext(ctx).
			WithArguments(ctx, oldDeployedResources, testClient).
			Should(Succeed())
	})

	It("Should remove module resources from status.synced", func() {
		EventuallyDeclarativeStatusShould(
			ctx, key, testClient,
			BeInState(shared.StateReady),
			HaveConditionWithStatus(ConditionTypeResources, apimetav1.ConditionTrue),
			HaveConditionWithStatus(ConditionTypeInstallation, apimetav1.ConditionTrue),
		)

		Eventually(validateOldResourcesAreRemovedFromStatusSynced, Timeout, Interval).
			WithArguments(ctx, testClient, key, oldDeployedResources).
			WithContext(ctx).
			Should(Succeed())
	})

	AfterAll(func() {
		cancel()
		Expect(env.Stop()).To(Succeed())
	})
})

func isResourceFoundInSynced(res *unstructured.Unstructured, resource shared.Resource) bool {
	return resource == shared.Resource{
		Name:      res.GetName(),
		Namespace: res.GetNamespace(),
		GroupVersionKind: apimetav1.GroupVersionKind{
			Group:   res.GroupVersionKind().Group,
			Version: res.GroupVersionKind().Version,
			Kind:    res.GetKind(),
		},
	}
}

func StartDeclarativeReconcilerForRun(
	ctx context.Context,
	runID string,
	cfg *rest.Config,
	options ...Option,
) *Reconciler {
	var (
		namespace  = fmt.Sprintf("%s-%s", "test", runID)
		finalizer  = fmt.Sprintf("%s-%s", FinalizerDefault, runID)
		mgr        ctrl.Manager
		reconciler reconcile.Reconciler
		err        error
	)
	mgr, err = ctrl.NewManager(
		cfg, ctrl.Options{
			// these bind addresses cause conflicts when run concurrently so we disable them
			HealthProbeBindAddress: "0",
			Metrics: metricsserver.Options{
				BindAddress: "0",
			},
			Scheme: k8sclientscheme.Scheme,
		},
	)
	Expect(err).ToNot(HaveOccurred())
	reconciler = NewFromManager(
		mgr, &declarativetest.TestAPI{},
		append(
			options,
			WithNamespace(namespace, true),
			WithFinalizer(finalizer),
			// we overwrite the manifest cache directory with the test run directory so its automatically cleaned up
			// we ensure uniqueness implicitly, as runID is used to randomize the ManifestName in SpecResolver
			WithManifestCache(filepath.Join(testDir, "declarative-test-cache")),
			// we have to use a custom ready check that only checks for existence of an object since the default
			// readiness check will not work without dedicated control loops in env test. E.g. by default
			// deployments are not started/set to ready. We can check if the resource was created by reconciler.
			WithClientCacheKey(),
			WithCustomReadyCheck(NewExistsReadyCheck()),
			WithCustomResourceLabels(k8slabels.Set{testRunLabel: runID}),
			WithPeriodicConsistencyCheck(2*time.Second),
		)...,
	)
	// in case there is any leak of CRs from another test run, but this is most likely never necessary
	testWatchPredicate, err := predicate.LabelSelectorPredicate(
		apimetav1.LabelSelector{MatchLabels: k8slabels.Set{testRunLabel: runID}},
	)
	Expect(err).ToNot(HaveOccurred())
	Expect(
		ctrl.NewControllerManagedBy(mgr).WithEventFilter(testWatchPredicate).
			WithOptions(
				ctrlruntime.Options{
					RateLimiter: workqueue.NewMaxOfRateLimiter(
						&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(30), 200)},
					),
				},
			).
			For(&declarativetest.TestAPI{}).Complete(reconciler),
	).To(Succeed())
	go func() {
		Expect(mgr.Start(ctx)).To(Succeed(), "failed to run manager")
	}()

	recon, ok := reconciler.(*Reconciler)
	Expect(ok).To(BeTrue())
	return recon
}

func StatusOnCluster(ctx context.Context, key client.ObjectKey,
	testClient client.Client,
) shared.Status {
	obj := &declarativetest.TestAPI{}
	Expect(testClient.Get(ctx, key, obj)).To(Succeed())
	return obj.GetStatus()
}

func WithClientCacheKey() WithClientCacheKeyOption {
	cacheKey := func(ctx context.Context, resource Object) (any, bool) {
		return client.ObjectKeyFromObject(resource), true
	}
	return WithClientCacheKeyOption{ClientCacheKeyFn: cacheKey}
}

func StartEnv() (*envtest.Environment, *rest.Config) {
	env := &envtest.Environment{
		CRDs:   []*apiextensionsv1.CustomResourceDefinition{testAPICRD},
		Scheme: k8sclientscheme.Scheme,
	}
	cfg, err := env.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	return env, cfg
}

func GetTestClient(cfg *rest.Config) client.Client {
	testClient, err := client.New(cfg, client.Options{Scheme: k8sclientscheme.Scheme})
	Expect(testClient.List(context.Background(), &declarativetest.TestAPIList{})).To(
		Succeed(), "Test API should be available",
	)
	Expect(err).NotTo(HaveOccurred())
	customResourceNamespace.SetResourceVersion("")
	Expect(testClient.Create(context.Background(), customResourceNamespace)).To(Succeed())

	return testClient
}

func validateOldResourcesNotLongerDeployed(ctx context.Context,
	resources internal.ManifestResources,
	testClient client.Client,
) error {
	for _, res := range resources.Items {
		currentRes := &unstructured.Unstructured{}
		currentRes.SetGroupVersionKind(res.GroupVersionKind())
		currentRes.SetName(res.GetName())
		currentRes.SetNamespace(customResourceNamespace.Name)
		err := testClient.Get(ctx, client.ObjectKeyFromObject(currentRes), currentRes)
		if !util.IsNotFound(err) {
			return ErrOldResourcesStillDeployed
		}
	}
	return nil
}

func validateOldResourcesAreRemovedFromStatusSynced(
	ctx context.Context, testClient client.Client, key client.ObjectKey,
	resources internal.ManifestResources,
) error {
	var obj declarativetest.TestAPI
	Expect(testClient.Get(ctx, key, &obj)).To(Succeed())
	for _, res := range resources.Items {
		for _, s := range obj.Status.Synced {
			if isResourceFoundInSynced(res, s) {
				return ErrOldResourcesStillInSynced
			}
		}
	}
	return nil
}
