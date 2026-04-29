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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"
)

const (
	apiPkg          = "github.com/kyma-project/lifecycle-manager/api/v1beta2"
	crdVersion      = "v1beta2"
	outputFilePerms = 0o600
)

var (
	errNoSpec          = errors.New("no spec")
	errNoSpecVersions  = errors.New("no spec.versions")
	errNoSchema        = errors.New("no schema for version")
	errNoOpenAPISchema = errors.New("no openAPIV3Schema for version")
	errVersionNotFound = errors.New("version not found")
	errNoRepoRoot      = errors.New("could not find repo root (no go.mod found)")
)

type crdEntry struct {
	crdFile string
	kind    string
}

func main() {
	repoRoot, err := findRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error finding repo root: %v\n", err)
		os.Exit(1)
	}

	entries := []crdEntry{
		{crdFile: "config/crd/bases/operator.kyma-project.io_kymas.yaml", kind: "Kyma"},
		{crdFile: "config/crd/bases/operator.kyma-project.io_manifests.yaml", kind: "Manifest"},
		{crdFile: "config/crd/bases/operator.kyma-project.io_modulereleasemetas.yaml", kind: "ModuleReleaseMeta"},
		{crdFile: "config/crd/bases/operator.kyma-project.io_moduletemplates.yaml", kind: "ModuleTemplate"},
		{crdFile: "config/crd/bases/operator.kyma-project.io_watchers.yaml", kind: "Watcher"},
	}

	definitions := map[string]any{}

	for _, entry := range entries {
		schema, err := extractSchema(filepath.Join(repoRoot, entry.crdFile), crdVersion)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error extracting schema from %s: %v\n", entry.crdFile, err)
			os.Exit(1)
		}
		definitions[restFriendlyName(apiPkg, entry.kind)] = schema
	}

	swagger := map[string]any{
		"swagger": "2.0",
		"info": map[string]any{
			"title":   "lifecycle-manager",
			"version": crdVersion,
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
	if err := os.WriteFile(outFile, append(out, '\n'), outputFilePerms); err != nil {
		fmt.Fprintf(os.Stderr, "error writing %s: %v\n", outFile, err)
		os.Exit(1)
	}

	fmt.Fprintln(os.Stdout, "wrote", outFile)
}

// extractSchema reads a CRD YAML and returns the openAPIV3Schema for the given version.
func extractSchema(crdFile, version string) (map[string]any, error) {
	data, err := os.ReadFile(crdFile)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", crdFile, err)
	}

	var crd map[string]any
	if err := yaml.Unmarshal(data, &crd); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", crdFile, err)
	}

	spec, ok := crd["spec"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w in %s", errNoSpec, crdFile)
	}

	versions, ok := spec["versions"].([]any)
	if !ok {
		return nil, fmt.Errorf("%w in %s", errNoSpecVersions, crdFile)
	}

	for _, raw := range versions {
		ver, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if ver["name"] != version {
			continue
		}
		schema, ok := ver["schema"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%w %s in %s", errNoSchema, version, crdFile)
		}
		openAPIV3Schema, ok := schema["openAPIV3Schema"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%w %s in %s", errNoOpenAPISchema, version, crdFile)
		}
		stripSwaggerV2Incompatible(openAPIV3Schema)
		return openAPIV3Schema, nil
	}

	return nil, fmt.Errorf("%w %s in %s", errVersionNotFound, version, crdFile)
}

// stripSwaggerV2Incompatible recursively removes properties that are valid in OpenAPI v3
// but not accepted by the Swagger v2 parser used by applyconfiguration-gen.
func stripSwaggerV2Incompatible(schema map[string]any) {
	delete(schema, "nullable")
	for _, key := range []string{"properties", "additionalProperties"} {
		if sub, ok := schema[key].(map[string]any); ok {
			for _, val := range sub {
				if nested, ok := val.(map[string]any); ok {
					stripSwaggerV2Incompatible(nested)
				}
			}
		}
	}
	for _, key := range []string{"items", "not"} {
		if sub, ok := schema[key].(map[string]any); ok {
			stripSwaggerV2Incompatible(sub)
		}
	}
	for _, key := range []string{"allOf", "anyOf", "oneOf"} {
		if arr, ok := schema[key].([]any); ok {
			for _, val := range arr {
				if nested, ok := val.(map[string]any); ok {
					stripSwaggerV2Incompatible(nested)
				}
			}
		}
	}
}

// restFriendlyName converts a Go package path and type name to a Swagger definition key.
// For example, "github.com/kyma-project/lifecycle-manager/api/v1beta2" and "ModuleReleaseMeta"
// produces "com.github.kyma-project.lifecycle-manager.api.v1beta2.ModuleReleaseMeta".
func restFriendlyName(pkg, kind string) string {
	parts := strings.Split(pkg, "/")
	domainParts := strings.Split(parts[0], ".")
	for left, right := 0, len(domainParts)-1; left < right; left, right = left+1, right-1 {
		domainParts[left], domainParts[right] = domainParts[right], domainParts[left]
	}
	reversed := strings.Join(domainParts, ".")
	rest := strings.Join(parts[1:], ".")
	return reversed + "." + rest + "." + kind
}

// findRepoRoot walks up from the current directory until it finds a go.mod file.
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errNoRepoRoot
		}
		dir = parent
	}
}
