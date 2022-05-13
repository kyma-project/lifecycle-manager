package release

import (
	"fmt"
	"github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/adapter"
)

type ChannelSwitch interface {
	GetOld() v1alpha1.Channel
	GetNew() v1alpha1.Channel
	IssueChannelChangeEvent()
}

type oldNewChannelSwitch struct {
	old         v1alpha1.Channel
	new         v1alpha1.Channel
	eventIssuer func()
}

func (k *oldNewChannelSwitch) GetOld() v1alpha1.Channel {
	return k.old
}

func (k *oldNewChannelSwitch) GetNew() v1alpha1.Channel {
	return k.new
}

func (k *oldNewChannelSwitch) IssueChannelChangeEvent() {
	k.eventIssuer()
}

func New(old, new v1alpha1.Channel, adapter adapter.Eventing) ChannelSwitch {
	rel := &oldNewChannelSwitch{
		old: old,
		new: new,
	}
	rel.eventIssuer = func() {
		if old != new {
			adapter("Normal", "ChannelUpdate", fmt.Sprintf("channel update: %s -> %s", rel.old, rel.new))
		}
	}
	return rel
}
