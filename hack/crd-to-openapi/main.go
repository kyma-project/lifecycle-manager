/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// crd-to-openapi converts CRD YAML files to a Swagger v2 JSON schema file
// suitable for use with applyconfiguration-gen's --openapi-schema flag.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"
)

type crdEntry struct {
	crdFile string
	version string
	kind    string
	goPkg   string
}

func main() {
	repoRoot, err := findRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error finding repo root: %v\n", err)
		os.Exit(1)
	}

	entries := []crdEntry{
		{
			crdFile: "config/crd/bases/operator.kyma-project.io_modulereleasemetas.yaml",
			version: "v1beta2",
			kind:    "ModuleReleaseMeta",
			goPkg:   "github.com/kyma-project/lifecycle-manager/api/v1beta2",
		},
		{
			crdFile: "config/crd/bases/operator.kyma-project.io_moduletemplates.yaml",
			version: "v1beta2",
			kind:    "ModuleTemplate",
			goPkg:   "github.com/kyma-project/lifecycle-manager/api/v1beta2",
		},
	}

	definitions := map[string]any{}

	for _, e := range entries {
		schema, err := extractSchema(filepath.Join(repoRoot, e.crdFile), e.version)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error extracting schema from %s: %v\n", e.crdFile, err)
			os.Exit(1)
		}
		key := restFriendlyName(e.goPkg, e.kind)
		definitions[key] = schema
	}

	swagger := map[string]any{
		"swagger": "2.0",
		"info": map[string]any{
			"title":   "lifecycle-manager",
			"version": "v1beta2",
		},
		"paths":       map[string]any{},
		"definitions": definitions,
	}

	out, err := json.MarshalIndent(swagger, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error marshaling JSON: %v\n", err)
		os.Exit(1)
	}

	outFile := filepath.Join(repoRoot, "hack", "crd-to-openapi", "openapi.json")
	if err := os.WriteFile(outFile, append(out, '\n'), 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "error writing %s: %v\n", outFile, err)
		os.Exit(1)
	}

	fmt.Printf("wrote %s\n", outFile)
}

// extractSchema reads a CRD YAML and returns the openAPIV3Schema for the given version.
func extractSchema(crdFile, version string) (map[string]any, error) {
	data, err := os.ReadFile(crdFile) //nolint:gosec // path is constructed from known static values
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", crdFile, err)
	}

	var crd map[string]any
	if err := yaml.Unmarshal(data, &crd); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", crdFile, err)
	}

	spec, ok := crd["spec"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("no spec in %s", crdFile)
	}

	versions, ok := spec["versions"].([]any)
	if !ok {
		return nil, fmt.Errorf("no spec.versions in %s", crdFile)
	}

	for _, v := range versions {
		ver, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if ver["name"] != version {
			continue
		}
		schema, ok := ver["schema"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("no schema for version %s in %s", version, crdFile)
		}
		openAPIV3Schema, ok := schema["openAPIV3Schema"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("no openAPIV3Schema for version %s in %s", version, crdFile)
		}
		return openAPIV3Schema, nil
	}

	return nil, fmt.Errorf("version %s not found in %s", version, crdFile)
}

// restFriendlyName converts a Go package path + type name to a Swagger definition key.
// e.g. "github.com/kyma-project/lifecycle-manager/api/v1beta2" + "ModuleReleaseMeta"
// → "io.github.kyma-project.lifecycle-manager.api.v1beta2.ModuleReleaseMeta"
func restFriendlyName(goPkg, kind string) string {
	// Split on "/"
	parts := strings.Split(goPkg, "/")
	// First part is the domain (e.g. "github.com") — reverse it
	domainParts := strings.Split(parts[0], ".")
	for i, j := 0, len(domainParts)-1; i < j; i, j = i+1, j-1 {
		domainParts[i], domainParts[j] = domainParts[j], domainParts[i]
	}
	reversed := strings.Join(domainParts, ".")
	rest := strings.Join(parts[1:], ".")
	return reversed + "." + rest + "." + kind
}

// findRepoRoot walks up from the current directory until it finds a go.mod file.
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find repo root (no go.mod found)")
		}
		dir = parent
	}
}
