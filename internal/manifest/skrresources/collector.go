package skrresources

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jellydator/ttlcache/v3"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/manifestclient"
)

const (
	knownManagersDefault = string(manifestclient.DefaultFieldOwner) + ";" +
		shared.OperatorName + ";" +
		"k3s" // Applied in k3s environments.
	knownManagersEnvVar = "KLM_EXPERIMENTAL_KNOWN_MANAGERS"
	knownManagersRegexp = `^[a-zA-Z][a-zA-Z0-9.:_/-]{1,127}$`

	frequencyCacheTTLDefault = 60 * 5 // 5 minutes
	frequencyCacheTTLEnvVar  = "KLM_EXPERIMENTAL_FREQUENCY_CACHE_TTL"
	frequencyCacheTTLRegexp  = `^[1-9][0-9]{1,3}$`

	managedFieldsAnalysisLabelEnvVar = "KLM_EXPERIMENTAL_MANAGED_FIELDS_ANALYSIS_LABEL"
)

var (
	allowedManagers = getAllowedManagers() //nolint:gochecknoglobals // list of managers is a global configuration
	emitCache       = newEmitCache()       //nolint:gochecknoglobals // singleton cache is used to prevent emitting the same log multiple times in a short period
)

type LogCollectorEntry struct {
	ObjectName      string                         `json:"objectName"`
	ObjectNamespace string                         `json:"objectNamespace"`
	ObjectGVK       string                         `json:"objectGvk"`
	ManagedFields   []apimetav1.ManagedFieldsEntry `json:"managedFields"`
}

// Implements ManagedFieldsCollector interface, emits the colloected data to the log stream.
type LogCollector struct {
	key     string
	owner   client.FieldOwner
	entries []LogCollectorEntry
}

func NewLogCollector(key string, owner client.FieldOwner) *LogCollector {
	return &LogCollector{
		key:     key,
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
				ManagedFields:   slices.Clone(remoteObj.GetManagedFields()),
			}
			c.entries = append(c.entries, newEntry)
			return
		}
	}
}

func (c *LogCollector) Emit(ctx context.Context) error {
	if len(c.entries) > 0 {
		if emitCache.Has(c.key) {
			logger := logf.FromContext(ctx, "owner", c.owner)
			logger.V(internal.TraceLogLevel).Info("Unknown managers detection skipped (frequency)")
			return nil
		}
		emitCache.Set(c.key, true, ttlcache.DefaultTTL)

		jsonSer, err := json.MarshalIndent(c.entries, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to serialize managed field data: %w", err)
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

// compressAndBase64 compresses the input byte slice using gzip and encodes it to base64 so that it can be logged as a string.
func compressAndBase64(in []byte) (string, error) {
	var buf bytes.Buffer
	archive := gzip.NewWriter(&buf)

	_, err := archive.Write(in)
	if err != nil {
		return "", fmt.Errorf("failed to write to gzip archive: %w", err)
	}

	if err := archive.Close(); err != nil {
		return "", fmt.Errorf("failed to close gzip archive: %w", err)
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func isUnknownManager(manager string) bool {
	return !slices.Contains(allowedManagers, manager)
}

// allowedManagers returns either a list configured in the KLM_RECONCILECONFIG_KNOWN_MANAGERS environment variable or the default list.
// The values must be separated by semicolons and are case-sensitive!
func getAllowedManagers() []string {
	configured := os.Getenv(knownManagersEnvVar)
	if configured == "" {
		return splitBySemicolons(knownManagersDefault)
	} else {
		rxp := regexp.MustCompile(knownManagersRegexp)
		configuredValues := splitBySemicolons(configured)
		res := []string{}
		for _, name := range configuredValues {
			if rxp.MatchString(name) {
				res = append(res, name)
			}
		}
		return res
	}
}

func getCacheTTL() int {
	var res int = frequencyCacheTTLDefault

	if configured := os.Getenv(frequencyCacheTTLEnvVar); configured != "" {
		rxp := regexp.MustCompile(frequencyCacheTTLRegexp)
		if rxp.MatchString(configured) {
			if parsed, err := strconv.Atoi(configured); err == nil {
				res = parsed
			}
		}
	}

	return res
}

func newEmitCache() *ttlcache.Cache[string, bool] {
	cache := ttlcache.New[string, bool](ttlcache.WithTTL[string, bool](time.Duration(getCacheTTL()) * time.Second))
	go cache.Start()
	return cache
}

func splitBySemicolons(value string) []string {
	return strings.Split(value, ";")
}

// Implements ManagedFieldsCollector interface, does nothing.
type nopCollector struct{}

func (c nopCollector) Collect(ctx context.Context, obj client.Object) {
}

func (c nopCollector) Emit(ctx context.Context) error {
	return nil
}

func getManagedFieldsAnalysisLabel() string {
	return os.Getenv(managedFieldsAnalysisLabelEnvVar)
}
