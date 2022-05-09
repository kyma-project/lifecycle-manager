package release

import (
	"fmt"
	"golang.org/x/mod/semver"
)

type EventAdapter func(eventtype, reason, message string)

type Kyma struct {
	Old  string `json:"old"`
	New  string `json:"new"`
	Type Type   `json:"type"`
}

func NewKyma(old, new string, adapter EventAdapter) *Kyma {
	rel := &Kyma{
		Old: old,
		New: new,
	}

	rel.calculateReleaseType(adapter)

	return rel
}

func (k *Kyma) calculateReleaseType(eventSender EventAdapter) {
	compared := semver.Compare(k.Old, k.New)
	var t Type
	if compared < 0 {
		if k.Old == "" {
			t = Install
			eventSender("Normal", "ReconciliationUpgrade", fmt.Sprintf("Initial Installation: %s", k.New))
		} else {
			t = Upgrade
			eventSender("Normal", "ReconciliationUpgrade", fmt.Sprintf("Upgrade from %s to %s", k.Old, k.New))
		}
	} else if compared > 0 {
		t = Downgrade
		eventSender("Normal", "ReconciliationDowngrade", fmt.Sprintf("Downgrade from %s to %s", k.Old, k.New))
	} else {
		t = Update
		eventSender("Normal", "ReconciliationUpdate", fmt.Sprintf("Update Active Release %s", k.Old))
	}

	k.Type = t
}
