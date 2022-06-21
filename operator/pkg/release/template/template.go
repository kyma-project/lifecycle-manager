package template

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	operatorv1alpha1 "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Finder interface {
	Lookup(ctx context.Context) (*LookupResult, error)
}

type LookupResult struct {
	Template   *operatorv1alpha1.ModuleTemplate
	forChannel *operatorv1alpha1.Channel
}

type LookupResultsByName map[string]*LookupResult

func GetTemplates(ctx context.Context, c client.Reader, k *operatorv1alpha1.Kyma, cache Cache) (LookupResultsByName, error) {
	templates := make(LookupResultsByName)
	for _, component := range k.Spec.Components {
		template, err := NewCachedChannelBasedFinder(c, component, k.Spec.Channel, cache).Lookup(ctx)
		if err != nil {
			return nil, err
		}
		templates[component.Name] = template
	}
	return templates, nil
}

func AreTemplatesOutdated(logger *logr.Logger, k *operatorv1alpha1.Kyma, templates LookupResultsByName) bool {
	// this is a shortcut as we already know templates are outdated when the generation changes
	if k.GetGeneration() != k.Status.ObservedGeneration {
		logger.Info("new kyma spec, setting template status outdated")
		return true
	}
	// in the case that the kyma spec did not change, we only have to verify that all desired templates are still referenced in the latest spec generation
	for componentName, lookupResult := range templates {
		for _, condition := range k.Status.Conditions {
			if condition.Reason == componentName && lookupResult != nil {
				if lookupResult.Template.GetGeneration() != condition.TemplateInfo.Generation || lookupResult.Template.Spec.Channel != condition.TemplateInfo.Channel {
					logger.Info("detected outdated template",
						"condition", condition.Reason,
						"template", lookupResult.Template.Name,
						"templateGeneration", lookupResult.Template.GetGeneration(),
						"previousGeneration", condition.TemplateInfo.Generation,
						"templateChannel", lookupResult.Template.Spec.Channel,
						"previousChannel", condition.TemplateInfo.Channel,
					)
					return true
				}
			}
		}
	}
	return false
}

func MoreThanOneTemplateCandidateErr(component operatorv1alpha1.ComponentType, candidateTemplates []operatorv1alpha1.ModuleTemplate) error {
	candidates := make([]string, len(candidateTemplates))
	for i, candidate := range candidateTemplates {
		candidates[i] = candidate.GetName()
	}
	return fmt.Errorf("more than one config map template found for component: %s, candidates: %v", component.Name, candidates)
}

func GetTemplateCacheKey(module string, channel operatorv1alpha1.Channel) string {
	return fmt.Sprintf("%s/%s", module, channel)
}

func GetUnstructuredComponentFromTemplate(templates LookupResultsByName, componentName string, kyma *operatorv1alpha1.Kyma) (*unstructured.Unstructured, error) {
	lookupResult := templates[componentName]
	if lookupResult == nil {
		return nil, fmt.Errorf("could not find template %s for resource %s",
			componentName, client.ObjectKeyFromObject(kyma))
	}

	desiredComponentStruct := &lookupResult.Template.Spec.Data
	desiredComponentStruct.SetName(componentName + kyma.Name)
	desiredComponentStruct.SetNamespace(kyma.GetNamespace())

	return desiredComponentStruct.DeepCopy(), nil
}
