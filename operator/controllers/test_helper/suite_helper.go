package test_helper

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

const (
	defaultBufferSize = 2048
)

func ParseRemoteCRDs(testCrdURLs []string) ([]*apiextv1.CustomResourceDefinition, error) {
	var crds []*apiextv1.CustomResourceDefinition
	for _, testCrdURL := range testCrdURLs {
		_, err := url.Parse(testCrdURL)
		if err != nil {
			return nil, err
		}
		request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, testCrdURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed pulling content for URL (%s) :%w", testCrdURL, err)
		}
		client := &http.Client{Timeout: time.Second * 2}
		response, err := client.Do(request)
		if err != nil {
			return nil, err
		}
		if response.StatusCode != http.StatusOK {
			//nolint:goerr113
			return nil, fmt.Errorf("failed pulling content for URL (%s) with status code: %d",
				testCrdURL, response.StatusCode)
		}
		defer response.Body.Close()
		decoder := yaml.NewYAMLOrJSONDecoder(response.Body, defaultBufferSize)
		for {
			crd := &apiextv1.CustomResourceDefinition{}
			err = decoder.Decode(crd)
			if err == nil {
				crds = append(crds, crd)
			}
			if errors.Is(err, io.EOF) {
				break
			}
		}
	}
	return crds, nil
}
