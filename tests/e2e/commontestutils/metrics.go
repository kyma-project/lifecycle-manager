package commontestutils

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
)

const (
	kcpMetricsPort = 9081
	skrMetricsPort = 2112
)

var ErrMetricNotFound = errors.New("metric was not found")

func GetMaintenanceWindowGauge(ctx context.Context) (int, error) {
	bodyString, err := getKCPMetricsBody(ctx)
	if err != nil {
		return 0, err
	}

	re := regexp.MustCompile(metrics.MetricMaintenanceWindowConfigReadSuccess + ` (\d+)`)
	return parseCount(re, bodyString)
}

func GetFipsModeGauge(ctx context.Context) (int, error) {
	bodyString, err := getKCPMetricsBody(ctx)
	if err != nil {
		return 0, err
	}

	re := regexp.MustCompile(metrics.MetricFipsMode + ` (\d+)`)
	return parseCount(re, bodyString)
}

func GetKymaStateMetricCount(ctx context.Context, kymaName string, state shared.State) (int, error) {
	bodyString, err := getKCPMetricsBody(ctx)
	if err != nil {
		return 0, err
	}

	re := getKymaStateMetricRegex(kymaName, state)
	return parseCount(re, bodyString)
}

func getKymaStateMetricRegex(kymaName string, state shared.State) *regexp.Regexp {
	return regexp.MustCompile(fmt.Sprintf(`%s{instance_id="[^"]+",kyma_name="%s",shoot="[^"]+",state="%s"} (\d+)`,
		metrics.MetricKymaState,
		kymaName, string(state)))
}

func AssertKymaStateMetricNotFound(ctx context.Context, kymaName string, state shared.State) error {
	bodyString, err := getKCPMetricsBody(ctx)
	if err != nil {
		return err
	}

	re := getKymaStateMetricRegex(kymaName, state)
	match := re.FindStringSubmatch(bodyString)
	if len(match) < 1 {
		return ErrMetricNotFound
	}

	return nil
}

func GetRequeueReasonCount(ctx context.Context,
	requeueReason, requeueType string,
) (int, error) {
	bodyString, err := getKCPMetricsBody(ctx)
	if err != nil {
		return 0, err
	}
	re := regexp.MustCompile(
		metrics.MetricRequeueReason + `{requeue_reason="` + requeueReason +
			`",requeue_type="` + requeueType +
			`"} (\d+)`)
	return parseCount(re, bodyString)
}

func IsManifestRequeueReasonCountIncreased(ctx context.Context, requeueReason, requeueType string) (bool,
	error,
) {
	bodyString, err := getKCPMetricsBody(ctx)
	if err != nil {
		return false, err
	}
	re := regexp.MustCompile(
		metrics.MetricRequeueReason + `{requeue_reason="` + requeueReason +
			`",requeue_type="` + requeueType +
			`"} (\d+)`)
	count, err := parseCount(re, bodyString)
	return count >= 1, err
}

func GetModuleStateMetricCount(ctx context.Context, kymaName, moduleName string, state shared.State) (int, error) {
	bodyString, err := getKCPMetricsBody(ctx)
	if err != nil {
		return 0, err
	}
	regex := regexp.MustCompile(
		fmt.Sprintf(`%s{instance_id="[^"]+",kyma_name="%s",module_name="%s",shoot="[^"]+",state="%s"} (\d+)`,
			metrics.MetricModuleState,
			kymaName,
			moduleName, string(state)),
	)
	return parseCount(regex, bodyString)
}

func PurgeMetricsAreAsExpected(ctx context.Context,
	timeShouldBeMoreThan float64,
	expectedRequests int,
) bool {
	correctCount := false
	correctTime := false
	bodyString, err := getKCPMetricsBody(ctx)
	if err != nil {
		return false
	}
	reg := regexp.MustCompile(`lifecycle_mgr_purgectrl_time ([0-9]*\.?[0-9]+)`)
	match := reg.FindStringSubmatch(bodyString)

	if len(match) > 1 {
		duration, err := strconv.ParseFloat(match[1], 64)
		if err == nil && duration > timeShouldBeMoreThan {
			correctTime = true
		}
	}

	reg = regexp.MustCompile(`lifecycle_mgr_purgectrl_requests_total (\d+)`)
	match = reg.FindStringSubmatch(bodyString)

	if len(match) > 1 {
		count, err := strconv.Atoi(match[1])
		if err == nil && count == expectedRequests {
			correctCount = true
		}
	}

	return correctTime && correctCount
}

func GetSelfSignedCertNotRenewMetricsGauge(ctx context.Context, kymaName string) (int, error) {
	bodyString, err := getKCPMetricsBody(ctx)
	if err != nil {
		return 0, err
	}

	re := regexp.MustCompile(fmt.Sprintf(`%s{%s="%s"} (\d+)`, metrics.MetricSelfSignedCertNotRenew,
		metrics.KymaNameLabel,
		kymaName))
	return parseCount(re, bodyString)
}

func GetMandatoryModuleTemplateCountMetric(ctx context.Context) (int, error) {
	bodyString, err := getKCPMetricsBody(ctx)
	if err != nil {
		return 0, err
	}

	re := regexp.MustCompile(metrics.MetricMandatoryTemplateCount + ` (\d+)`)
	return parseCount(re, bodyString)
}

func GetMandatoryModuleStateMetric(ctx context.Context, kymaName, moduleName, state string) (int, error) {
	bodyString, err := getKCPMetricsBody(ctx)
	if err != nil {
		return 0, err
	}

	re := regexp.MustCompile(fmt.Sprintf(`%s{kyma_name="%s",module_name="%s",state="%s"} (\d+)`,
		metrics.MetricMandatoryModuleState, kymaName, moduleName, state))
	return parseCount(re, bodyString)
}

func GetWatcherFailedKcpTotalMetric(ctx context.Context) (int, error) {
	metricsBody, err := getSKRMetricsBody(ctx)
	if err != nil {
		return 0, err
	}
	regex := regexp.MustCompile(`watcher_failed_kcp_total{error_reason="failed-request"} (\d+)`)
	return parseCount(regex, metricsBody)
}

func getKCPMetricsBody(ctx context.Context) (string, error) {
	return getMetricsBody(ctx, kcpMetricsPort)
}

func getSKRMetricsBody(ctx context.Context) (string, error) {
	return getMetricsBody(ctx, skrMetricsPort)
}

func getMetricsBody(ctx context.Context, port int) (string, error) {
	clnt := &http.Client{}
	url := fmt.Sprintf("http://localhost:%d/metrics", port)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("request to metrics endpoint :%w", err)
	}
	response, err := clnt.Do(request)
	if err != nil {
		return "", fmt.Errorf("response from metrics endpoint :%w", err)
	}
	defer response.Body.Close()
	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("response body:%w", err)
	}
	bodyString := string(bodyBytes)

	return bodyString, nil
}

func parseCount(re *regexp.Regexp, bodyString string) (int, error) {
	match := re.FindStringSubmatch(bodyString)
	if len(match) > 1 {
		count, err := strconv.Atoi(match[1])
		if err != nil {
			return 0, fmt.Errorf("parse count:%w", err)
		}
		return count, nil
	}

	return 0, nil
}
