package ocmextensions

import (
	"fmt"

	"github.com/open-component-model/ocm/pkg/contexts/ocm/cpi"
	"github.com/open-component-model/ocm/pkg/runtime"
)

// HelmChartRepositoryType is the access type of a oci blob in the current component descriptor manifest.
const HelmChartRepositoryType = "helmChartRepository"

//nolint:gochecknoinits
func init() {
	cpi.RegisterAccessType(
		cpi.NewAccessSpecType(
			HelmChartRepositoryType, &HelmChartRepositoryAccess{},
		),
	)
}

var _ cpi.AccessSpec = (*HelmChartRepositoryAccess)(nil)

// New creates a new LocalOCIBlob accessor.
// Deprecated: Use LocalBlob.
func New(repoURL, name, version string) *HelmChartRepositoryAccess {
	return &HelmChartRepositoryAccess{
		ObjectVersionedType: runtime.NewVersionedObjectType(HelmChartRepositoryType),
		HelmChartRepoURL:    repoURL,
		HelmChartName:       name,
		HelmChartVersion:    version,
	}
}

// HelmChartRepositoryAccess describes the access for a blob that is stored in the component descriptors oci manifest.
type HelmChartRepositoryAccess struct {
	runtime.ObjectVersionedType `json:",inline"`
	// Digest is the digest of the targeted content.
	HelmChartRepoURL string `json:"helmChartRepoUrl"`
	HelmChartName    string `json:"helmChartName"`
	HelmChartVersion string `json:"helmChartVersion"`
}

func (a *HelmChartRepositoryAccess) Describe(ctx cpi.Context) string {
	return fmt.Sprintf("Helm Chart %s, Verstion %s, Repo %s", a.HelmChartName, a.HelmChartVersion, a.HelmChartRepoURL)
}

func (a *HelmChartRepositoryAccess) IsLocal(context cpi.Context) bool {
	panic("implement me")
}

func (a *HelmChartRepositoryAccess) AccessMethod(c cpi.ComponentVersionAccess) (cpi.AccessMethod, error) {
	return c.AccessMethod(a)
}

func (*HelmChartRepositoryAccess) GetType() string {
	return HelmChartRepositoryType
}
