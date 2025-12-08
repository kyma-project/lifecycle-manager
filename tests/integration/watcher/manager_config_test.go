package watcher_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/tests/integration"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Watcher Manager Configuration", func() {
	Context("kyma_watcher.yaml configuration", func() {
		It("should have spec.manager equal to shared.OperatorName", func() {
			// Read the kyma_watcher.yaml file
			projectRoot := integration.GetProjectRoot()
			watcherFilePath := filepath.Join(projectRoot, "config", "watcher", "kyma_watcher.yaml")

			fileContent, err := os.ReadFile(watcherFilePath)
			Expect(err).ToNot(HaveOccurred(), "should be able to read kyma_watcher.yaml")

			// Parse YAML - handle multiple documents
			decoder := yaml.NewDecoder(bytes.NewReader(fileContent))
			var watcherFound bool

			for {
				var doc map[string]interface{}
				err := decoder.Decode(&doc)
				if err != nil {
					break // End of documents
				}

				// Check if this document is a Watcher
				kind, hasKind := doc["kind"].(string)
				if !hasKind || kind != "Watcher" {
					continue
				}

				var watcher v1beta2.Watcher
				jsonBytes, err := json.Marshal(doc)
				Expect(err).ToNot(HaveOccurred())

				err = json.Unmarshal(jsonBytes, &watcher)
				Expect(err).ToNot(HaveOccurred(), "should be able to parse Watcher CR")

				watcherFound = true

				Expect(watcher.Spec.Manager).To(Equal(shared.OperatorName),
					"kyma_watcher.yaml should have spec.manager set to lifecycle-manager")

				Expect(watcher.GetManagerName()).To(Equal(shared.OperatorName),
					"GetManagerName() should return spec.manager when it's set")

				break
			}

			Expect(watcherFound).To(BeTrue(), "should find a Watcher CR in kyma_watcher.yaml")
		})

		It("should have consistent manager configuration between spec field and GetManagerName()", func() {
			// Test backward compatibility: when spec.Manager is empty, fall back to label
			watcherWithLabel := &v1beta2.Watcher{
				ObjectMeta: apimetav1.ObjectMeta{
					Labels: map[string]string{
						shared.ManagedBy: "fallback-manager",
					},
				},
				Spec: v1beta2.WatcherSpec{
					Manager: "", // Empty, should fall back to label
				},
			}

			Expect(watcherWithLabel.GetManagerName()).To(Equal("fallback-manager"),
				"GetManagerName() should fall back to label when spec.Manager is empty")

			// Test priority: spec.Manager takes precedence over label
			watcherWithBoth := &v1beta2.Watcher{
				ObjectMeta: apimetav1.ObjectMeta{
					Labels: map[string]string{
						shared.ManagedBy: "label-manager",
					},
				},
				Spec: v1beta2.WatcherSpec{
					Manager: "spec-manager",
				},
			}

			Expect(watcherWithBoth.GetManagerName()).To(Equal("spec-manager"),
				"spec.Manager should take precedence over the label")
		})
	})
})
