package v1beta1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	declarative "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
	"helm.sh/helm/v3/pkg/strvals"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ChartInfo defines helm chart information.
type ChartInfo struct {
	ChartPath   string
	RepoName    string
	URL         string
	ChartName   string
	ReleaseName string
}

var ErrNoAuthSecretFound = errors.New("no auth secret found")

type ManifestSpecResolver struct {
	KCP *declarative.ClusterInfo

	*v1beta1.Codec

	ChartCache   string
	cachedCharts map[string]string
}

func NewManifestSpecResolver(kcp *declarative.ClusterInfo, codec *v1beta1.Codec) *ManifestSpecResolver {
	return &ManifestSpecResolver{
		KCP:          kcp,
		Codec:        codec,
		ChartCache:   os.TempDir(),
		cachedCharts: make(map[string]string),
	}
}

var (
	ErrRenderModeInvalid                   = errors.New("render mode is invalid")
	ErrInvalidObjectPassedToSpecResolution = errors.New("invalid object passed to spec resolution")
)

func (m *ManifestSpecResolver) Spec(ctx context.Context, obj declarative.Object) (*declarative.Spec, error) {
	manifest, ok := obj.(*v1beta1.Manifest)
	if !ok {
		return nil, fmt.Errorf(
			"invalid type %s: %w", reflect.TypeOf(obj),
			ErrInvalidObjectPassedToSpecResolution,
		)
	}

	specType, err := v1beta1.GetSpecType(manifest.Spec.Install.Source.Raw)
	if err != nil {
		return nil, err
	}

	keyChain, err := m.lookupKeyChain(ctx, manifest.Spec.Config)
	if err != nil {
		return nil, err
	}

	chartInfo, err := m.getChartInfoForInstall(ctx, manifest.Spec.Install, specType, keyChain)
	if err != nil {
		return nil, err
	}

	var mode declarative.RenderMode
	switch specType {
	case v1beta1.HelmChartType:
		mode = declarative.RenderModeHelm
	case v1beta1.OciRefType:
		mode = declarative.RenderModeHelm
	case v1beta1.KustomizeType:
		mode = declarative.RenderModeKustomize
	case v1beta1.NilRefType:
		return nil, fmt.Errorf("could not determine render mode for %s: %w",
			client.ObjectKeyFromObject(manifest), ErrRenderModeInvalid)
	}

	values, err := m.getValuesFromConfig(ctx, manifest.Spec.Config, manifest.Spec.Install.Name, keyChain)
	if err != nil {
		return nil, err
	}

	path := chartInfo.ChartPath
	if path == "" && chartInfo.URL != "" {
		path = chartInfo.URL

		if mode == declarative.RenderModeHelm {
			path, err = m.downloadAndCacheHelmChart(chartInfo)
			if err != nil {
				return nil, err
			}
		}
	}

	return &declarative.Spec{
		ManifestName: manifest.Spec.Install.Name,
		Path:         path,
		Values:       values,
		Mode:         mode,
	}, nil
}

func (m *ManifestSpecResolver) downloadAndCacheHelmChart(chartInfo *ChartInfo) (string, error) {
	filename := filepath.Join(m.ChartCache, chartInfo.ChartName)

	if cachedChart, ok := m.cachedCharts[filename]; !ok {
		getters := getter.All(cli.New())
		chart, err := repo.FindChartInRepoURL(
			chartInfo.URL,
			chartInfo.ChartName, "", "", "", "", getters,
		)
		if err != nil {
			return "", err
		}
		cachedChart, _, err := (&downloader.ChartDownloader{Getters: getters}).DownloadTo(
			chart, "", m.ChartCache,
		)
		if err != nil {
			return "", err
		}
		m.cachedCharts[filename] = cachedChart
		filename = cachedChart
	} else {
		filename = cachedChart
	}

	return filename, nil
}

func (m *ManifestSpecResolver) getValuesFromConfig(
	ctx context.Context, config v1beta1.ImageSpec, name string, keyChain authn.Keychain,
) (map[string]any, error) {
	var configs []any
	if config.Type.NotEmpty() { //nolint:nestif
		decodedConfig, err := DecodeUncompressedYAMLLayer(ctx, config, keyChain)
		if err != nil {
			// if EOF error, we should proceed without config
			if !errors.Is(err, io.EOF) {
				return nil, err
			}
		} else {
			var err error
			configs, err = ParseInstallConfigs(decodedConfig)
			if err != nil {
				return nil, fmt.Errorf("value parsing for %s encountered an err: %w", name, err)
			}
		}
	}

	// filter config for install
	chartValues, err := parseChartConfigAndValues(configs, name)
	if err != nil {
		return nil, err
	}
	return chartValues, nil
}

var (
	ErrConfigObjectInvalid = errors.New(".spec.config is invalid")
)

type ConfigsYaml struct {
	Configs []interface{}
}

func ParseInstallConfigs(decodedConfig interface{}) ([]interface{}, error) {
	var configs []interface{}
	decodedConfig = nil
	var configsValues ConfigsYaml
	configsValuesYamlObj, err := json.Marshal(decodedConfig)
	if err != nil {
		return nil, fmt.Errorf("reading install %s resulted in an error: %w", v1beta1.ManifestKind,
			ErrConfigObjectInvalid)
	}
	err = json.Unmarshal(configsValuesYamlObj, &configsValues)
	if err != nil {
		return nil, fmt.Errorf("reading install %s resulted in an error: %w", v1beta1.ManifestKind,
			ErrConfigObjectInvalid)
	}

	if configsValues.Configs != nil {
		configs = configsValues.Configs
	}

	return configs, nil
}

var (
	ErrUnsupportedInstallType = errors.New("install type is not supported")
	ErrEmptyInstallType       = errors.New("empty install type")
)

func (m *ManifestSpecResolver) getChartInfoForInstall(
	ctx context.Context,
	install v1beta1.InstallInfo,
	specType v1beta1.RefTypeMetadata,
	keyChain authn.Keychain,
) (*ChartInfo, error) {
	var err error
	switch specType {
	case v1beta1.HelmChartType:
		var helmChartSpec v1beta1.HelmChartSpec
		if err = m.Codec.Decode(install.Source.Raw, &helmChartSpec, specType); err != nil {
			return nil, err
		}

		return &ChartInfo{
			ChartName: helmChartSpec.ChartName,
			RepoName:  install.Name,
			URL:       helmChartSpec.URL,
		}, nil
	case v1beta1.OciRefType:
		var imageSpec v1beta1.ImageSpec
		if err = m.Codec.Decode(install.Source.Raw, &imageSpec, specType); err != nil {
			return nil, err
		}

		// extract helm chart from layer digest
		chartPath, err := GetPathFromExtractedTarGz(ctx, imageSpec, keyChain)
		if err != nil {
			return nil, err
		}

		return &ChartInfo{
			ChartName: install.Name,
			ChartPath: chartPath,
		}, nil
	case v1beta1.KustomizeType:
		var kustomizeSpec v1beta1.KustomizeSpec
		if err = m.Codec.Decode(install.Source.Raw, &kustomizeSpec, specType); err != nil {
			return nil, err
		}

		return &ChartInfo{
			ChartName: install.Name,
			ChartPath: kustomizeSpec.Path,
			URL:       kustomizeSpec.URL,
		}, nil
	case v1beta1.NilRefType:
		return nil, ErrEmptyInstallType
	}

	return nil, fmt.Errorf(
		"%s is invalid: %w", specType, ErrUnsupportedInstallType,
	)
}

func parseChartConfigAndValues(
	configs []interface{}, name string,
) (map[string]interface{}, error) {
	valuesString, err := getConfigAndValuesForInstall(configs, name)
	if err != nil {
		return nil, fmt.Errorf("manifest encountered an error while parsing chart config: %w", err)
	}

	values := map[string]interface{}{}
	if err := strvals.ParseInto(valuesString, values); err != nil {
		return nil, err
	}

	return values, nil
}

var (
	ErrConfigDoesNotExist        = errors.New("config object does not exist")
	ErrConfigOverridesDoNotExist = errors.New("config object overrides do not exist")
)

func getConfigAndValuesForInstall(configs []interface{}, name string) (
	string, error,
) {
	var defaultOverrides string

	for _, config := range configs {
		mappedConfig, configExists := config.(map[string]interface{})
		if !configExists {
			return "", fmt.Errorf(
				"reading install %s resulted in an error: %w", v1beta1.ManifestKind, ErrConfigDoesNotExist,
			)
		}
		if mappedConfig["name"] == name {
			defaultOverrides, configExists = mappedConfig["overrides"].(string)
			if !configExists {
				return "", fmt.Errorf(
					"reading install %s resulted in an error: %w",
					v1beta1.ManifestKind, ErrConfigOverridesDoNotExist,
				)
			}
			break
		}
	}
	return defaultOverrides, nil
}

func (m *ManifestSpecResolver) lookupKeyChain(
	ctx context.Context, imageSpec v1beta1.ImageSpec,
) (authn.Keychain, error) {
	var keyChain authn.Keychain
	var err error
	if imageSpec.CredSecretSelector != nil {
		if keyChain, err = GetAuthnKeychain(ctx, imageSpec, m.KCP.Client); err != nil {
			return nil, err
		}
	} else {
		keyChain = authn.DefaultKeychain
	}
	return keyChain, nil
}
