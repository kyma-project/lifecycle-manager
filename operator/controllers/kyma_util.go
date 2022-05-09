package controllers

import (
	"flag"
	operatorv1alpha1 "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"os"
	"path/filepath"
	"time"
)

func GetConfig() (*rest.Config, error) {
	// in-cluster config
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, err
	}

	// kubeconfig flag
	if flag.Lookup("kubeconfig") != nil {
		if kubeconfig := flag.Lookup("kubeconfig").Value.String(); kubeconfig != "" {
			return clientcmd.BuildConfigFromFlags("", kubeconfig)
		}
	}

	// env variable
	if len(os.Getenv("KUBECONFIG")) > 0 {
		return clientcmd.BuildConfigFromFlags("masterURL", os.Getenv("KUBECONFIG"))
	}

	// If no in-cluster config, try the default location in the user's home directory
	if home := homedir.HomeDir(); home != "" {
		return clientcmd.BuildConfigFromFlags("", filepath.Join(home, ".kube", "config"))
	}

	return nil, err
}

func GetConditionForComponent(kymaObj *operatorv1alpha1.Kyma, componentName string) (*operatorv1alpha1.KymaCondition, bool) {
	status := &kymaObj.Status
	for _, existingCondition := range status.Conditions {
		if existingCondition.Type == operatorv1alpha1.ConditionTypeReady && existingCondition.Reason == componentName {
			return &existingCondition, true
		}
	}
	return &operatorv1alpha1.KymaCondition{}, false
}

func AddConditionForComponents(kymaObj *operatorv1alpha1.Kyma, componentNames []string, conditionStatus operatorv1alpha1.KymaConditionStatus, message string) {
	status := &kymaObj.Status
	for _, componentName := range componentNames {
		condition, exists := GetConditionForComponent(kymaObj, componentName)
		if !exists {
			condition = &operatorv1alpha1.KymaCondition{
				Type:   operatorv1alpha1.ConditionTypeReady,
				Reason: componentName,
			}
			status.Conditions = append(status.Conditions, *condition)
		}
		condition.LastTransitionTime = &metav1.Time{Time: time.Now()}
		condition.Message = message
		condition.Status = conditionStatus

		for i, existingCondition := range status.Conditions {
			if existingCondition.Type == operatorv1alpha1.ConditionTypeReady && existingCondition.Reason == componentName {
				status.Conditions[i] = *condition
				break
			}
		}
	}
}
