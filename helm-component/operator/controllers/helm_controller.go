/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/gofrs/flock"
	"github.com/kyma-project/kyma-operator/helm-component/api/api/v1alpha1"
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
	"helm.sh/helm/v3/pkg/strvals"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"os"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strconv"
	"strings"
	"sync"
	"time"

	"helm.sh/helm/v3/pkg/action"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

// HelmReconciler reconciles a Helm object
type HelmReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=helms,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=helms/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=helms/finalizers,verbs=update
func (r *HelmReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	helmObj := v1alpha1.Helm{}
	if err := r.Get(ctx, client.ObjectKey{Name: req.Name, Namespace: req.Namespace}, &helmObj); err != nil {
		panic(err)
	}
	if helmObj.Annotations == nil {
		helmObj.Annotations = make(map[string]string, 0)
	}
	//if helmObj.Annotations["operator.kyma-project.io/status"] == "" {
	//	helmObj.Annotations["operator.kyma-project.io/status"] = string(v1alpha1.HelmStateProcessing)
	//	if err := r.Client.Update(ctx, &helmObj); err != nil {
	//		panic(err)
	//	}
	//}
	if helmObj.Status.State == "" {
		helmObj.Status.State = v1alpha1.HelmStateProcessing
		if err := r.Status().Update(ctx, &helmObj); err != nil {
			panic(err)
		}
	}

	var (
		repoName         = helmObj.Spec.RepoName
		url              = helmObj.Spec.Url
		chartName        = helmObj.Spec.ChartName
		releaseName      = helmObj.Spec.ReleaseName
		releaseNamespace = helmObj.Spec.ReleaseNamespace
	)

	settings := cli.New()

	create, err := strconv.ParseBool(helmObj.Spec.Create)
	if err != nil {
		panic(err)
	}

	if create {
		if err := r.AddHelmRepo(settings, repoName, url, logger); err != nil {
			panic(err)
		}

		if exists, err := r.GetChart(releaseName, settings); !exists {
			logger.Info(err.Error(), "response", "chart not found, installing..")
			if err := r.InstallChart(settings, logger, releaseName, releaseNamespace, repoName, chartName, map[string]string{}); err != nil {
				panic(err)
			}
		} else {
			logger.Info("release already exists", "chart name", releaseName)
		}

		if err := r.RepoUpdate(settings); err != nil {
			panic(err)
		}
	} else {
		if err := r.UninstallChart(settings, releaseName, logger); err != nil {
			panic(err)
		}
	}

	helmObj = v1alpha1.Helm{}
	if err := r.Get(ctx, client.ObjectKey{Name: req.Name, Namespace: req.Namespace}, &helmObj); err != nil {
		panic(err)
	}

	//if helmObj.Annotations["operator.kyma-project.io/status"] != string(v1alpha1.HelmStateReady) {
	//	helmObj.Annotations["operator.kyma-project.io/status"] = string(v1alpha1.HelmStateReady)
	//	if err := r.Client.Update(ctx, &helmObj); err != nil {
	//		panic(err)
	//	}
	//}

	if helmObj.Status.State == v1alpha1.HelmStateProcessing {
		helmObj.Status.State = v1alpha1.HelmStateReady
		if err := r.Status().Update(ctx, &helmObj); err != nil {
			panic(err)
		}
	}

	//actionConfig := new(action.Configuration)
	//
	//if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {
	//	fmt.Sprintf(format, v)
	//}); err != nil {
	//	panic(err)
	//}
	//
	//iCli := action.NewInstall(actionConfig)
	//chartPath, err := iCli.ChartPathOptions.LocateChart(fmt.Sprintf("%s/%s", repoName, chartName), settings)
	//if err != nil {
	//	panic(err)
	//}
	//
	//logger.Info("", "chartPath", chartPath)
	//
	////chartPath, err := iCli.LocateChart("https://github.com/kubernetes/ingress-nginx/releases/download/helm-chart-4.0.6/ingress-nginx-4.0.6.tgz", settings)
	////if err != nil {
	////	panic(err)
	////}
	//chart, err := loader.Load(chartPath)
	//if err != nil {
	//	panic(err)
	//}
	//iCli.Namespace = releaseNamespace
	//iCli.ReleaseName = releaseName
	//rel, err := iCli.Run(chart, nil)
	//if err != nil {
	//	panic(err)
	//}
	//fmt.Println("Successfully installed release: ", rel.Name)
	//
	//// check Release Status, feel free to run it in a go routine along the deletion logic
	//upCli := action.NewUpgrade(actionConfig)
	//upgradedRel, err := r.pollAndUpdate(rel, upCli) // see if its better to just run that code here directly
	//
	//// if we still on pending, then delete it
	//if upgradedRel.Info.Status.IsPending() {
	//	unCli := action.NewUninstall(actionConfig)
	//	res, err := unCli.Run(rel.Name)
	//	if err != nil {
	//		panic(err)
	//	}
	//	logger.Info(res.Info)
	//}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HelmReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Helm{}).
		Complete(r)
}

//func (r *HelmReconciler) pollAndUpdate(originalRel *release.Release, upgradeCli *action.Upgrade) (*release.Release, error) {
//	if !originalRel.Info.Status.IsPending() {
//		return originalRel, nil
//	}
//	c := time.Tick(10 * time.Second) // we gonna time it out besides checking repeatedly
//	rel := originalRel
//	var err error
//	for _ = range c {
//		//check the status and try and upgrade
//		if rel.Info.Status.IsPending() { // https://pkg.go.dev/helm.sh/helm/v3@v3.5.4/pkg/release#Status.IsPending
//			// run the upgrade command you have
//			// its this function: https://github.com/helm/helm/blob/main/pkg/action/upgrade.go#L111
//			rel, err = upgradeCli.Run(originalRel.Name, originalRel.Chart, originalRel.Config)
//			if err != nil {
//				panic(err)
//			}
//		}
//	}
//	return rel, nil
//}

func (r *HelmReconciler) GetChart(releaseName string, settings *cli.EnvSettings) (bool, error) {
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {
		fmt.Sprintf(format, v)
	}); err != nil {
		panic(err)
	}
	client := action.NewGet(actionConfig)
	result, err := client.Run(releaseName)
	if err != nil {

		return false, err
	}
	return result != nil, nil
}

func (r *HelmReconciler) InstallChart(settings *cli.EnvSettings, logger logr.Logger, releaseName string, namespace string, repoName string, chartName string, args map[string]string) error {
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {
		fmt.Sprintf(format, v)
	}); err != nil {
		panic(err)
	}
	client := action.NewInstall(actionConfig)

	if client.Version == "" && client.Devel {
		client.Version = ">0.0.0-0"
	}
	//name, chart, err := client.NameAndChart(args)
	client.ReleaseName = releaseName
	client.Namespace = namespace
	cp, err := client.ChartPathOptions.LocateChart(fmt.Sprintf("%s/%s", repoName, chartName), settings)
	if err != nil {
		panic(err)
	}

	logger.Info("", "CHART PATH", cp)

	p := getter.All(settings)
	valueOpts := &values.Options{}
	vals, err := valueOpts.MergeValues(p)
	if err != nil {
		panic(err)
	}

	// Add args
	if err := strvals.ParseInto(args["set"], vals); err != nil {
		panic(err)
	}

	// Check chart dependencies to make sure all are present in /charts
	chartRequested, err := loader.Load(cp)
	if err != nil {
		panic(err)
	}

	if chartRequested.Metadata.Type != "" && chartRequested.Metadata.Type != "application" {
		return fmt.Errorf("%s charts are not installable", chartRequested.Metadata.Type)
	}

	if req := chartRequested.Metadata.Dependencies; req != nil {
		// If CheckDependencies returns an error, we have unfulfilled dependencies.
		// As of Helm 2.4.0, this is treated as a stopping condition:
		// https://github.com/helm/helm/issues/2209
		if err := action.CheckDependencies(chartRequested, req); err != nil {
			if client.DependencyUpdate {
				man := &downloader.Manager{
					Out:              os.Stdout,
					ChartPath:        cp,
					Keyring:          client.ChartPathOptions.Keyring,
					SkipUpdate:       false,
					Getters:          p,
					RepositoryConfig: settings.RepositoryConfig,
					RepositoryCache:  settings.RepositoryCache,
				}
				if err := man.Update(); err != nil {
					panic(err)
				}
			} else {
				panic(err)
			}
		}
	}

	client.Namespace = settings.Namespace()
	release, err := client.Run(chartRequested, vals)
	if err != nil {
		panic(err)
	}
	fmt.Println(release.Manifest)
	return nil
}

// RepoUpdate
func (r *HelmReconciler) RepoUpdate(settings *cli.EnvSettings) error {
	repoFile := settings.RepositoryConfig

	f, err := repo.LoadFile(repoFile)
	if os.IsNotExist(errors.Cause(err)) || len(f.Repositories) == 0 {
		return fmt.Errorf("no repositories found. You must add one before updating")
	}
	var repos []*repo.ChartRepository
	for _, cfg := range f.Repositories {
		r, err := repo.NewChartRepository(cfg, getter.All(settings))
		if err != nil {
			panic(err)
		}
		repos = append(repos, r)
	}

	fmt.Printf("Hang tight while we grab the latest from your chart repositories...\n")
	var wg sync.WaitGroup
	for _, re := range repos {
		wg.Add(1)
		go func(re *repo.ChartRepository) {
			defer wg.Done()
			if _, err := re.DownloadIndexFile(); err != nil {
				fmt.Printf("...Unable to get an update from the %q chart repository (%s):\n\t%s\n", re.Config.Name, re.Config.URL, err)
			} else {
				fmt.Printf("...Successfully got an update from the %q chart repository\n", re.Config.Name)
			}
		}(re)
	}
	wg.Wait()
	fmt.Printf("Update Complete. ⎈ Happy Helming!⎈\n")
	return nil
}

// AddHelmRepo
func (r *HelmReconciler) AddHelmRepo(settings *cli.EnvSettings, repoName string, url string, logger logr.Logger) error {
	repoFile := settings.RepositoryConfig

	// File locking mechanism
	if err := os.MkdirAll(filepath.Dir(repoFile), os.ModePerm); err != nil && !os.IsExist(err) {
		panic(err)
	}
	fileLock := flock.New(strings.Replace(repoFile, filepath.Ext(repoFile), ".lock", 1))
	lockCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	locked, err := fileLock.TryLockContext(lockCtx, time.Second)
	if err == nil && locked {
		defer fileLock.Unlock()
	}

	b, err := ioutil.ReadFile(repoFile)
	if err != nil && !os.IsNotExist(err) {
		panic(err)
	}

	var f repo.File
	if err := yaml.Unmarshal(b, &f); err != nil {
		panic(err)
	}

	if f.Has(repoName) {
		logger.Info("helm repo already exists", "name", repoName)
	}

	c := repo.Entry{
		Name: repoName,
		URL:  url,
	}

	chartRepo, err := repo.NewChartRepository(&c, getter.All(settings))
	if err != nil {
		return fmt.Errorf("repository name (%s) already exists\n %w", repoName, err)
	}

	if _, err := chartRepo.DownloadIndexFile(); err != nil {
		return fmt.Errorf("looks like %s is not a valid chart repository or cannot be reached %w", url, err)
	}

	f.Update(&c)
	repoConfig := settings.RepositoryConfig
	if err := f.WriteFile(repoConfig, 0644); err != nil {
		return err
	}
	fmt.Printf("%q has been added to your repositories\n", repoName)
	return nil
}

// UninstallChart
func (r *HelmReconciler) UninstallChart(settings *cli.EnvSettings, releaseName string, logger logr.Logger) error {
	if exists, err := r.GetChart(releaseName, settings); !exists {
		logger.Info(err.Error(), "response", "chart already deleted")
		return nil
	}
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {
		fmt.Sprintf(format, v)
	}); err != nil {
		panic(err)
	}
	client := action.NewUninstall(actionConfig)
	response, err := client.Run(releaseName)
	if err != nil {
		panic(err)
	}
	logger.Info("", "response", response.Release.Info.Description)
	return nil
}
