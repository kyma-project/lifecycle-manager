package skrresources

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"

	"context"
	"encoding/json"
	"github.com/kyma-project/lifecycle-manager/internal"
	apimachineryv1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// Implements ManagedFieldsCollector interface, does nothing
type nopCollector struct{}

func (c nopCollector) Collect(ctx context.Context, obj client.Object) {
}

func (c nopCollector) Emit(ctx context.Context) error {
	return nil
}

type LogCollectorEntry struct {
	ObjectName      string
	ObjectNamespace string
	ObjectGVK       string
	ManagedFields   []apimachineryv1.ManagedFieldsEntry
}

// Implements ManagedFieldsCollector interface, emits the colloected data to the log stream
type LogCollector struct {
	owner   client.FieldOwner
	entries []LogCollectorEntry
}

func NewLogCollector(owner client.FieldOwner) *LogCollector {
	return &LogCollector{
		owner:   owner,
		entries: []LogCollectorEntry{},
	}
}

func (c *LogCollector) Collect(ctx context.Context, remoteObj client.Object) {
	managedFields := remoteObj.GetManagedFields()
	for _, mf := range managedFields {
		if isUnknownManager(mf.Manager) {
			newEntry := LogCollectorEntry{
				ObjectName:      remoteObj.GetName(),
				ObjectNamespace: remoteObj.GetNamespace(),
				ObjectGVK:       remoteObj.GetObjectKind().GroupVersionKind().String(),
				ManagedFields:   remoteObj.GetManagedFields(),
			}
			c.entries = append(c.entries, newEntry)
			return
		}
	}
}

func (c *LogCollector) Emit(ctx context.Context) error {
	if len(c.entries) > 0 {
		jsonSer, err := json.MarshalIndent(c.entries, "", "  ")
		if err != nil {
			return err
		}
		logData, err := compressAndBase64(jsonSer)
		if err != nil {
			return err
		}

		logger := logf.FromContext(ctx, "owner", c.owner)
		logger.V(internal.TraceLogLevel).Info("Unknown managers detected", "base64gzip", logData)
	}
	return nil
}

func compressAndBase64(in []byte) (string, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)

	_, err := zw.Write(in)
	if err != nil {
		return "", err
	}

	if err := zw.Close(); err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func isUnknownManager(manager string) bool {
	return manager != "unknown"
}
