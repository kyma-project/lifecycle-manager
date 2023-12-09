package testutils

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"

	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
)

func GetKymaStateMetricCount(ctx context.Context, kymaName, state string) (int, error) {
	bodyString, err := getMetricsBody(ctx)
	if err != nil {
		return 0, err
	}

	re := regexp.MustCompile(
		metrics.MetricKymaState + `{instance_id="[^"]+",kyma_name="` + kymaName +
			`",shoot="[^"]+",state="` + state +
			`"} (\d+)`)
	return parseCount(re, bodyString)
}

func GetModuleStateMetricCount(ctx context.Context, kymaName, moduleName, state string) (int, error) {
	bodyString, err := getMetricsBody(ctx)
	if err != nil {
		return 0, err
	}
	re := regexp.MustCompile(
		metrics.MetricModuleState + `{instance_id="[^"]+",kyma_name="` + kymaName +
			`",module_name="` + moduleName +
			`",shoot="[^"]+",state="` + state +
			`"} (\d+)`)
	return parseCount(re, bodyString)
}

func PurgeMetricsAreAsExpected(ctx context.Context,
	timeShouldBeMoreThan float64,
	expectedRequests int,
) bool {
	correctCount := false
	correctTime := false
	bodyString, err := getMetricsBody(ctx)
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
	bodyString, err := getMetricsBody(ctx)
	if err != nil {
		return 0, err
	}

	re := regexp.MustCompile(fmt.Sprintf(`%s{%s="%s"} (\d+)`, metrics.SelfSignedCertNotRenewMetrics,
		metrics.KymaNameLabel,
		kymaName))
	return parseCount(re, bodyString)
}

func getMetricsBody(ctx context.Context) (string, error) {
	clnt := &http.Client{}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost:9081/metrics", nil)
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
