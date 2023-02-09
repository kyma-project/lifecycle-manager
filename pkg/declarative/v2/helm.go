package v2

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/kube"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
)

const (
	ConditionTypeHelmCRDs               ConditionType   = "HelmCRDs"
	ConditionReasonHelmCRDsAreAvailable ConditionReason = "HelmCRDsAvailable"
)

func NewHelmRenderer(
	spec *Spec,
	clnt Client,
	options *Options,
) Renderer {
	return &Helm{
		recorder:   options.EventRecorder,
		chartPath:  spec.Path,
		values:     spec.Values,
		clnt:       clnt,
		crdChecker: NewHelmReadyCheck(clnt),
	}
}

type Helm struct {
	recorder record.EventRecorder
	clnt     Client

	chartPath string
	values    any

	crds kube.ResourceList

	crdChecker ReadyCheck
}

func (h *Helm) prerequisiteCondition(object metav1.Object) metav1.Condition {
	return metav1.Condition{
		Type:               string(ConditionTypeHelmCRDs),
		Reason:             string(ConditionReasonHelmCRDsAreAvailable),
		Status:             metav1.ConditionFalse,
		Message:            "CustomResourceDefinitions from chart 'crds' folder are installed and ready for use",
		ObservedGeneration: object.GetGeneration(),
	}
}

func (h *Helm) Initialize(obj Object) error {
	status := obj.GetStatus()

	prerequisiteExists := meta.FindStatusCondition(status.Conditions, h.prerequisiteCondition(obj).Type) != nil
	if !prerequisiteExists {
		meta.SetStatusCondition(&status.Conditions, h.prerequisiteCondition(obj))
		obj.SetStatus(status)
		return ErrConditionsNotYetRegistered
	}

	return nil
}

func (h *Helm) EnsurePrerequisites(ctx context.Context, obj Object) error {
	status := obj.GetStatus()

	if obj.GetDeletionTimestamp().IsZero() && meta.IsStatusConditionTrue(
		status.Conditions, h.prerequisiteCondition(obj).Type,
	) {
		return nil
	}

	chrt, err := loader.Load(h.chartPath)
	if err != nil {
		h.recorder.Event(obj, "Warning", "ChartLoading", err.Error())
		meta.SetStatusCondition(&status.Conditions, h.prerequisiteCondition(obj))
		obj.SetStatus(status.WithState(StateError).WithErr(err))
		return err
	}

	crds, err := getCRDs(h.clnt, chrt.CRDObjects())
	if err != nil {
		h.recorder.Event(obj, "Warning", "CRDParsing", err.Error())
		meta.SetStatusCondition(&status.Conditions, h.prerequisiteCondition(obj))
		obj.SetStatus(status.WithState(StateError).WithErr(err))
		return err
	}
	h.crds = crds

	if err := installCRDs(h.clnt, h.crds); err != nil {
		h.recorder.Event(obj, "Warning", "CRDInstallation", err.Error())
		meta.SetStatusCondition(&status.Conditions, h.prerequisiteCondition(obj))
		obj.SetStatus(status.WithState(StateError).WithErr(err))
		return fmt.Errorf("crds could not be installed: %w", err)
	}

	err = h.crdChecker.Run(ctx, h.clnt, obj, h.crds)

	if errors.Is(err, ErrResourcesNotReady) {
		h.recorder.Event(obj, "Normal", "CRDReadyCheck", "crds are not yet ready...")
		meta.SetStatusCondition(&status.Conditions, h.prerequisiteCondition(obj))
		obj.SetStatus(status.WithErr(ErrPrerequisitesNotFulfilled))
		return fmt.Errorf("crds are not yet ready: %w", ErrPrerequisitesNotFulfilled)
	}

	if err != nil {
		h.recorder.Event(obj, "Warning", "CRDReadyCheck", err.Error())
		meta.SetStatusCondition(&status.Conditions, h.prerequisiteCondition(obj))
		obj.SetStatus(status.WithState(StateError).WithErr(err))
		return fmt.Errorf("crds could not be checked for readiness: %w", err)
	}

	restMapper, _ := h.clnt.ToRESTMapper()
	meta.MaybeResetRESTMapper(restMapper)
	cond := h.prerequisiteCondition(obj)
	cond.Status = metav1.ConditionTrue
	h.recorder.Event(obj, "Normal", cond.Reason, cond.Message)
	meta.SetStatusCondition(&status.Conditions, cond)
	obj.SetStatus(status.WithOperation("CRDs are ready"))

	return nil
}

func (h *Helm) RemovePrerequisites(ctx context.Context, obj Object) error {
	status := obj.GetStatus()
	if err := NewConcurrentCleanup(h.clnt).Run(ctx, h.crds); errors.Is(err, ErrDeletionNotFinished) {
		waitingMsg := "waiting for crds to be uninstalled"
		h.recorder.Event(obj, "Normal", "CRDsUninstallation", waitingMsg)
		obj.SetStatus(status.WithOperation(waitingMsg))
		return err
	} else if err != nil {
		h.recorder.Event(obj, "Warning", "CRDsUninstallation", err.Error())
		obj.SetStatus(status.WithState(StateError).WithErr(err))
		return err
	}
	return nil
}

func (h *Helm) Render(ctx context.Context, obj Object) ([]byte, error) {
	status := obj.GetStatus()

	valuesAsMap, ok := h.values.(map[string]any)
	if !ok {
		h.recorder.Eventf(
			obj, "Warning", "HelmValuesParsing",
			"values are of type %s instead of %s and cannot be used. "+
				"if you are trying to pass custom objects, convert it to a generic map before passing.",
			reflect.TypeOf(h.values).String(),
			reflect.TypeOf(valuesAsMap).String(),
		)
		valuesAsMap = map[string]any{}
	}

	chrt, err := loader.Load(h.chartPath)
	if err != nil {
		h.recorder.Event(obj, "Warning", "ChartLoading", err.Error())
		meta.SetStatusCondition(&status.Conditions, h.prerequisiteCondition(obj))
		obj.SetStatus(status.WithState(StateError).WithErr(err))
		return nil, err
	}

	release, err := h.clnt.Install().RunWithContext(ctx, chrt, valuesAsMap)
	if err != nil {
		h.recorder.Event(obj, "Warning", "HelmRenderRun", err.Error())
		obj.SetStatus(status.WithState(StateError).WithErr(err))
		return nil, err
	}
	return []byte(release.Manifest), nil
}
