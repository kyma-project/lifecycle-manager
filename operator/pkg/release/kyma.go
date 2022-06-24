package release

import (
	"context"
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

func New(kyma *v1alpha1.Kyma, ctx context.Context) ChannelSwitch {
	o, n := kyma.Status.ActiveChannel, kyma.Spec.Channel
	recorder := adapter.RecorderFromContext(ctx)
	rel := &oldNewChannelSwitch{
		old: o,
		new: n,
	}
	rel.inProgress = func() {
		if o != n {
			recorder.Event(kyma, "Normal", "ChannelUpdateStart", fmt.Sprintf("defaultChannel update: %s -> %s", rel.old, rel.new))
		}
	}
	rel.success = func() {
		if o != n {
			recorder.Event(kyma, "Normal", "ChannelUpdateFinish", fmt.Sprintf("defaultChannel update to %s successful!", rel.new))
		}
	}
	return rel
}
