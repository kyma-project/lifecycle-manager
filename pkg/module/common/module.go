package common

import (
	"fmt"
	"hash/fnv"
	"strings"

	"github.com/go-logr/logr"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/channel"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type (
	Modules []*Module
	Module  struct {
		ModuleName string
		FQDN       string
		Version    string
		Template   *channel.ModuleTemplateTO
		client.Object
	}
)

func (m *Module) Logger(base logr.Logger) logr.Logger {
	return base.WithValues(
		"fqdn", m.FQDN,
		"module", m.GetName(),
		"channel", m.Template.Spec.Channel,
		"templateGeneration", m.Template.GetGeneration(),
	)
}

func (m *Module) ApplyLabelsAndAnnotations(kyma *v1beta2.Kyma) {
	lbls := m.GetLabels()
	if lbls == nil {
		lbls = make(map[string]string)
	}
	lbls[v1beta2.KymaName] = kyma.Name
	lbls[v1beta2.ChannelLabel] = m.Template.Spec.Channel
	m.SetLabels(lbls)

	anns := m.GetAnnotations()
	if anns == nil {
		anns = make(map[string]string)
	}
	anns[v1beta2.FQDN] = m.FQDN
	m.SetAnnotations(anns)
}

func (m *Module) ContainsExpectedOwnerReference(ownerName string) bool {
	if m.GetOwnerReferences() == nil {
		return false
	}
	for _, owner := range m.GetOwnerReferences() {
		if owner.Name == ownerName {
			return true
		}
	}
	return false
}

const maxModuleNameLength = validation.DNS1035LabelMaxLength

// CreateModuleName takes a FQDN and a prefix and generates a human-readable unique interpretation of
// a name combination.
// e.g. kyma-project.io/module/some-module and default-id => "default-id-some-module-34180237"
// e.g. domain.com/some-module and default-id => "default-id-some-module-1238916".
func CreateModuleName(fqdn, prefix, moduleName string) string {
	splitFQDN := strings.Split(fqdn, "/")
	lastPartOfFQDN := splitFQDN[len(splitFQDN)-1]
	hash := fnv.New32()
	_, _ = hash.Write([]byte(fqdn + moduleName))
	hashedFQDN := hash.Sum32()
	name := fmt.Sprintf("%s-%s-%v", prefix, lastPartOfFQDN, hashedFQDN)
	if len(name) >= maxModuleNameLength {
		name = name[:maxModuleNameLength-1]
	}
	return name
}
