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
	"sync"
	"time"

	"github.com/jellydator/ttlcache/v3"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/manifestclient"
)

const (
	knownManagersDefault = string(manifestclient.DefaultFieldOwner) + ";" +
		shared.OperatorName + ";" +
		"k3s" // Applied in k3s environments.
	knownManagersEnvVar = "KLM_EXPERIMENTAL_KNOWN_MANAGERS"
	knownManagersRegexp = `^[a-zA-Z][a-zA-Z0-9.:_/-]{1,127}$`

	frequencyLimiterTTLDefault = 60 * 5 // 5 minutes
	frequencyLimiterTTLEnvVar  = "KLM_EXPERIMENTAL_FREQUENCY_LIMITER_TTL"
	frequencyLimiterTTLRegexp  = `^[1-9][0-9]{1,3}$`

	managedFieldsAnalysisLabelEnvVar = "KLM_EXPERIMENTAL_MANAGED_FIELDS_ANALYSIS_LABEL"
)

var (
	allowedManagers           = getAllowedManagers()  //nolint:gochecknoglobals,revive // list of managers is a global configuration
	singletonFrequencyLimiter = newFrequencyLimiter() //nolint:gochecknoglobals,revive // singleton cache is used to prevent emitting the same log multiple times in a short period
)

type LogCollectorEntry struct {
	ObjectName      string                         `json:"objectName"`
	ObjectNamespace string                         `json:"objectNamespace"`
	ObjectGVK       string                         `json:"objectGvk"`
	ManagedFields   []apimetav1.ManagedFieldsEntry `json:"managedFields"`
}

// Implements skrresources.ManagedFieldsCollector interface, emits the collected data to the log stream.
// The collector is thread-safe.
// The collector is frequency-limited to prevent emitting entries for the same objectKey multiple times in a short time.
type LogCollector struct {
	objectKey        string
	frequencyLimiter *ttlcache.Cache[string, bool]
	owner            client.FieldOwner
	entries          []LogCollectorEntry
	mu               sync.Mutex
}

func NewLogCollector(key string, owner client.FieldOwner) *LogCollector {
	return &LogCollector{
		objectKey:        key,
		owner:            owner,
		entries:          []LogCollectorEntry{},
		frequencyLimiter: singletonFrequencyLimiter,
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
			c.safeAddEntry(newEntry)
			return
		}
	}
}

func (c *LogCollector) Emit(ctx context.Context) error {
	if c.frequencyLimiter.Has(c.objectKey) {
		logger := logf.FromContext(ctx, "owner", c.owner)
		logger.Info("Unknown managers detection skipped (frequency)")
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.entries) > 0 {
		c.frequencyLimiter.Set(c.objectKey, true, ttlcache.DefaultTTL)

		jsonSer, err := json.MarshalIndent(c.entries, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to serialize managed field data: %w", err)
		}
		logData, err := compressAndBase64(jsonSer)
		if err != nil {
			return err
		}

		logger := logf.FromContext(ctx, "owner", c.owner)
		logger.Info("Unknown managers detected", "base64gzip", logData)
	}
	return nil
}

// safeAddEntry adds a new entry to the collector's entries slice in a thread-safe way.
func (c *LogCollector) safeAddEntry(entry LogCollectorEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = append(c.entries, entry)
}

// compressAndBase64 compresses the input byte slice using gzip and encodes it to base64
// so that it can be logged as a string.
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

// allowedManagers returns either a list configured in the
// KLM_RECONCILECONFIG_KNOWN_MANAGERS environment variable or the default list.
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

func getFrequencyLimiterTTL() int {
	res := frequencyLimiterTTLDefault
	if configured := os.Getenv(frequencyLimiterTTLEnvVar); configured != "" {
		rxp := regexp.MustCompile(frequencyLimiterTTLRegexp)
		if rxp.MatchString(configured) {
			if parsed, err := strconv.Atoi(configured); err == nil {
				res = parsed
			}
		}
	}

	return res
}

func newFrequencyLimiter() *ttlcache.Cache[string, bool] {
	cache := ttlcache.New(ttlcache.WithTTL[string, bool](time.Duration(getFrequencyLimiterTTL()) * time.Second))
	go cache.Start()
	return cache
}

func splitBySemicolons(value string) []string {
	return strings.Split(value, ";")
}

func getManagedFieldsAnalysisLabel() string {
	return os.Getenv(managedFieldsAnalysisLabelEnvVar)
}
