package controllers_test

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func parseRemoteCRDs(testCrdURLs []string) ([]*apiextv1.CustomResourceDefinition, error) {
	var crds []*apiextv1.CustomResourceDefinition
	for _, testCrdURL := range testCrdURLs {
		_, err := url.Parse(testCrdURL)
		if err != nil {
			return nil, err
		}
		resp, err := http.Get(testCrdURL) //nolint:gosec
		if err != nil {
			return nil, fmt.Errorf("failed pulling content for URL (%s) :%w", testCrdURL, err)
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed pulling content for URL (%s) with status code: %d", testCrdURL, resp.StatusCode)
		}
		defer resp.Body.Close()
		decoder := yaml.NewYAMLOrJSONDecoder(resp.Body, defaultBufferSize)
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
