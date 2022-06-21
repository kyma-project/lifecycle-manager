package release

import (
	"fmt"
	"github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/adapter"
)

type ChannelSwitch interface {
	GetOld() v1alpha1.Channel
	GetNew() v1alpha1.Channel
	IssueChannelChangeInProgress()
	IssueChannelChangeSuccess()
}

type oldNewChannelSwitch struct {
	old        v1alpha1.Channel
	new        v1alpha1.Channel
	inProgress func()
	success    func()
}

func (k *oldNewChannelSwitch) GetOld() v1alpha1.Channel {
	return k.old
}

func (k *oldNewChannelSwitch) GetNew() v1alpha1.Channel {
	return k.new
}

func (k *oldNewChannelSwitch) IssueChannelChangeInProgress() {
	k.inProgress()
}

func (k *oldNewChannelSwitch) IssueChannelChangeSuccess() {
	k.success()
}

func New(old, new v1alpha1.Channel, adapter adapter.Eventing) ChannelSwitch {
	rel := &oldNewChannelSwitch{
		old: old,
		new: new,
	}
	rel.inProgress = func() {
		if old != new {
			adapter("Normal", "ChannelUpdateStart", fmt.Sprintf("defaultChannel update: %s -> %s", rel.old, rel.new))
		}
	}
	rel.success = func() {
		if old != new {
			adapter("Normal", "ChannelUpdateFinish", fmt.Sprintf("defaultChannel update to %s successful!", rel.new))
		}
	}
	return rel
}
