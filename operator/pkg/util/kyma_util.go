package util

import (
	"errors"
	"fmt"
	ocm "github.com/gardener/component-spec/bindings-go/apis/v2"
	"github.com/gardener/component-spec/bindings-go/codec"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	operatorv1alpha1 "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/labels"
	"github.com/kyma-project/kyma-operator/operator/pkg/release"
	errwrap "github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Modules map[string]*Module
type Module struct {
	Name             string
	Template         *operatorv1alpha1.ModuleTemplate
	TemplateOutdated bool
	*unstructured.Unstructured
	Settings []operatorv1alpha1.Settings
}

func (m *Module) Channel() operatorv1alpha1.Channel {
	return m.Template.Spec.Channel
}

func SetComponentCRLabels(unstructuredCompCR *unstructured.Unstructured, componentName string, channel operatorv1alpha1.Channel, kymaName string) {
	labelMap := unstructuredCompCR.GetLabels()
	if labelMap == nil {
		labelMap = make(map[string]string)
	}
	labelMap[labels.ControllerName] = componentName
	labelMap[labels.Channel] = string(channel)
	labelMap[labels.KymaName] = kymaName
	unstructuredCompCR.SetLabels(labelMap)
}

func CopySettingsToUnstructuredFromResource(resource *unstructured.Unstructured, settings []operatorv1alpha1.Settings) {
	if len(settings) > 0 {
		resource.Object["spec"].(map[string]interface{})["customStates"] = settings
	}
}

func ParseTemplates(kyma *operatorv1alpha1.Kyma, templates release.TemplatesInChannels) (Modules, error) {
	// First, we fetch the component spec from the template and use it to resolve it into an arbitrary object
	// (since we do not know which component we are dealing with)
	modules := make(Modules)
	for _, component := range kyma.Spec.Components {
		template := templates[component.Name]
		if template == nil {
			return nil, fmt.Errorf("could not find template %s for resource %s",
				component.Name, client.ObjectKeyFromObject(kyma))
		}
		if module, err := GetUnstructuredComponentFromTemplate(template, component.Name, kyma); err != nil {
			return nil, err
		} else {
			modules[component.Name] = &Module{
				Template:         template.Template,
				TemplateOutdated: template.Outdated,
				Unstructured:     module,
				Settings:         component.Settings,
			}
		}
	}
	return modules, nil
}

func GetUnstructuredComponentFromTemplate(template *release.TemplateInChannel, componentName string, kyma *operatorv1alpha1.Kyma) (*unstructured.Unstructured, error) {
	if template.Template.Spec.Descriptor.String() != "" {
		var descriptor ocm.ComponentDescriptor
		if err := codec.Decode(template.Template.Spec.Descriptor.Raw, &descriptor); err != nil {
			return nil, errwrap.Wrap(err, "error while decoding the descriptor")
		}
		repositoryCtx := descriptor.GetEffectiveRepositoryContext()

		switch repositoryCtx.GetType() {
		case ocm.OCIRegistryType:
			ociRepo := &ocm.OCIRegistryRepository{}
			if err := repositoryCtx.DecodeInto(ociRepo); err != nil {
				return nil, errwrap.Wrap(err, "error while decoding the repository context into OCIRegistryRepository")
			}

			imageURL := ociRepo.BaseURL
			switch ociRepo.ComponentNameMapping {
			case ocm.OCIRegistryURLPathMapping:
				imageURL = fmt.Sprintf("%s/component-descriptors/%s:%s", imageURL, descriptor.GetName(), descriptor.GetVersion())
			default:
				return nil, errors.New(fmt.Sprintf("unrecognized componentNameMapping %s", ociRepo.ComponentNameMapping))
			}

			// get a component by its identity via selectors
			resource, err := descriptor.GetResourceByIdentity(ocm.Identity{"name": componentName})
			if err != nil {
				return nil, errwrap.Wrap(err, "error while decoding the resource for the CRD Identity")
			}
			switch resource.GetType() {
			case "yaml":
				access := resource.Access
				switch access.GetType() {
				case ocm.LocalOCIBlobType:
					ociAccess := &ocm.LocalOCIBlobAccess{}
					if err := access.DecodeInto(ociAccess); err != nil {
						return nil, errwrap.Wrap(err, "error while decoding the access into OCIRegistryRepository")
					}
					img, err := crane.Pull(imageURL)
					if err != nil {
						return nil, errwrap.Wrap(err, "error while pulling the image from the image reference")
					}
					hash, err := v1.NewHash(ociAccess.Digest)
					if err != nil {
						return nil, errwrap.Wrap(err, "error, digest not supported")
					}
					layer, err := img.LayerByDigest(hash)
					if err != nil {
						return nil, errwrap.Wrap(err, "error, layer could not be read from digest")
					}
					layerData, err := layer.Uncompressed()
					if err != nil {
						return nil, errwrap.Wrap(err, "error, layer could not be read")
					}

					desiredComponentStruct := &unstructured.Unstructured{}
					if err := yaml.NewYAMLOrJSONDecoder(layerData, 2048).Decode(desiredComponentStruct); err != nil {
						return nil, errwrap.Wrap(err, "layer could not be parsed")
					}
					desiredComponentStruct.SetName(componentName + kyma.Name)
					desiredComponentStruct.SetNamespace(kyma.GetNamespace())
					return desiredComponentStruct.DeepCopy(), nil
				default:
					return nil, errors.New(fmt.Sprintf("access type %s not supported", access.GetType()))
				}
			default:
				return nil, errors.New(fmt.Sprintf("resource type %s not supported", resource.GetType()))
			}

		default:
			return nil, errors.New("OCI Registry is the only supported repository Context")
		}
	}
	desiredComponentStruct := &template.Template.Spec.Data
	desiredComponentStruct.SetName(componentName + kyma.Name)
	desiredComponentStruct.SetNamespace(kyma.GetNamespace())
	return desiredComponentStruct.DeepCopy(), nil
}
