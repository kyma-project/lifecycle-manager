package release

import (
	"fmt"
	"github.com/kyma-project/kyma-operator/operator/pkg/adapter"
	"golang.org/x/mod/semver"
)

type Type string

const (
	Upgrade   Type = "upgrade"
	Downgrade Type = "downgrade"
	Install   Type = "install"
	Update    Type = "update"
)

type Release interface {
	GetOld() string
	GetNew() string
	GetType() Type
	IssueReleaseEvent()
}

type kyma struct {
	old         string
	new         string
	t           Type
	eventIssuer func()
}

func (k *kyma) GetOld() string {
	return k.old
}

func (k *kyma) GetNew() string {
	return k.new
}

func (k *kyma) GetType() Type {
	return k.t
}

func (k *kyma) IssueReleaseEvent() {
	k.eventIssuer()
}

func New(old, new string, adapter adapter.Eventing) Release {
	rel := &kyma{
		old: old,
		new: new,
	}
	rel.calculateReleaseType(adapter)
	return rel
}

func (k *kyma) calculateReleaseType(eventSender adapter.Eventing) {
	compared := semver.Compare(k.old, k.new)
	if compared < 0 {
		if k.old == "" {
			k.t = Install
			k.eventIssuer = func() {
				eventSender("Normal", "ReconciliationInstall", fmt.Sprintf("Initial Installation: %s", k.new))
			}
		} else {
			k.t = Upgrade
			k.eventIssuer = func() {
				eventSender("Normal", "ReconciliationUpgrade", fmt.Sprintf("Upgrade from %s to %s", k.old, k.new))
			}
		}
	} else if compared > 0 {
		k.t = Downgrade
		k.eventIssuer = func() {
			eventSender("Normal", "ReconciliationDowngrade", fmt.Sprintf("Downgrade from %s to %s", k.old, k.new))
		}
	} else {
		k.t = Update
		k.eventIssuer = func() {
			eventSender("Normal", "ReconciliationUpdate", fmt.Sprintf("Update Active Release %s", k.old))
		}
	}
}
