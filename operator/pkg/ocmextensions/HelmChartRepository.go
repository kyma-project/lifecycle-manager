package ocmextensions

import ocm "github.com/gardener/component-spec/bindings-go/apis/v2"

// HelmChartRepositoryType is the access type of a oci blob in the current component descriptor manifest.
const HelmChartRepositoryType = "helmChartRepository"

var _ ocm.TypedObjectAccessor = &HelmChartRepositoryAccess{}

// NewLocalOCIBlobAccess creates a new LocalOCIBlob accessor.
func NewLocalOCIBlobAccess(repoUrl, name, version string) *HelmChartRepositoryAccess {
	return &HelmChartRepositoryAccess{
		ObjectType: ocm.ObjectType{
			Type: HelmChartRepositoryType,
		},
		HelmChartName:    name,
		HelmChartVersion: version,
		HelmChartRepoURL: repoUrl,
	}
}

// HelmChartRepositoryAccess describes the access for a blob that is stored in the component descriptors oci manifest.
type HelmChartRepositoryAccess struct {
	ocm.ObjectType `json:",inline"`
	// Digest is the digest of the targeted content.
	HelmChartRepoURL string `json:"helmChartRepoUrl"`
	HelmChartName    string `json:"helmChartName"`
	HelmChartVersion string `json:"helmChartVersion"`
}

func (*HelmChartRepositoryAccess) GetType() string {
	return HelmChartRepositoryType
}
