package internal

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	yamlUtil "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/kyma-project/lifecycle-manager/pkg/types"
)

const (
	YamlDecodeBufferSize            = 2048
	OthersReadExecuteFilePermission = 0o755
	DebugLogLevel                   = 2
	TraceLogLevel                   = 3
	ParseRetries                    = 3
)

var (
	ErrPathContainsDoubleDot      = errors.New("path contains '..', which is illegal")
	ErrPathContainsDriveSeperator = errors.New("path contains ':', which is illegal")
	ErrPathIsAbsolute             = errors.New("path is absolute, which is illegal")
	ErrParseInconsistent          = errors.New("parse manifest is inconsistent")
)

func CleanFilePathJoin(root, destDir string) (string, error) {
	// On Windows, this is a drive separator. On UNIX-like, this is the path list separator.
	// In neither case do we want to trust a TAR that contains these.
	if strings.Contains(destDir, ":") {
		return "", ErrPathContainsDriveSeperator
	}

	// The Go tar library does not convert separators for us.
	// We assume here, as we do elsewhere, that `\\` means a Windows path.
	destDir = strings.ReplaceAll(destDir, "\\", "/")

	// We want to alert the user that something bad was attempted. Cleaning it
	// is not a good practice.
	for _, part := range strings.Split(destDir, "/") {
		if part == ".." {
			return "", ErrPathContainsDoubleDot
		}
	}

	// If a path is absolute, the creator of the TAR is doing something shady.
	if path.IsAbs(destDir) {
		return "", ErrPathIsAbsolute
	}

	newPath := filepath.Join(root, filepath.Clean(destDir))

	return filepath.ToSlash(newPath), nil
}

func ConsistencyParseManifest(manifest string) (*ManifestResources, error) {
	resources, err := ParseManifestStringToObjects(manifest)
	if err != nil {
		return nil, err
	}
	for i := 0; i < ParseRetries; i++ {
		temp, err := ParseManifestStringToObjects(manifest)
		if err != nil {
			return nil, err
		}
		if len(temp.Items) != len(resources.Items) {
			return nil, ErrParseInconsistent
		}
	}
	return resources, nil
}

func ParseManifestStringToObjects(manifest string) (*ManifestResources, error) {
	objects := &ManifestResources{}
	reader := yamlUtil.NewYAMLReader(bufio.NewReader(strings.NewReader(manifest)))
	for {
		rawBytes, err := reader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return objects, nil
			}

			return nil, fmt.Errorf("invalid YAML doc: %w", err)
		}
		rawBytes = bytes.TrimSpace(rawBytes)
		unstructuredObj := unstructured.Unstructured{}
		if err := yaml.Unmarshal(rawBytes, &unstructuredObj); err != nil {
			objects.Blobs = append(objects.Blobs, append(bytes.TrimPrefix(rawBytes, []byte("---\n")), '\n'))
		}

		if len(rawBytes) == 0 || bytes.Equal(rawBytes, []byte("null")) || len(unstructuredObj.Object) == 0 {
			continue
		}

		objects.Items = append(objects.Items, &unstructuredObj)
	}
}

func GetYamlFileContent(filePath string) (interface{}, error) {
	var fileContent interface{}
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	if file != nil {
		if err = yamlUtil.NewYAMLOrJSONDecoder(file, YamlDecodeBufferSize).Decode(&fileContent); err != nil {
			return nil, fmt.Errorf("reading content from file path %s: %w", filePath, err)
		}
		err = file.Close()
	}

	return fileContent, err
}

func WriteToFile(filePath string, bytes []byte) error {
	// create directory
	if err := os.MkdirAll(filepath.Dir(filePath), fs.ModePerm); err != nil {
		return err
	}

	// create file
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("file creation at path %s caused an error: %w", filePath, err)
	}

	// write to file
	if _, err = file.Write(bytes); err != nil {
		return fmt.Errorf("writing file to path %s caused an error: %w", filePath, err)
	}
	return file.Close()
}

func GetResourceLabel(resource client.Object, labelName string) (string, error) {
	labels := resource.GetLabels()
	labelValue, ok := labels[labelName]
	if !ok {
		return "", &types.LabelNotFoundError{
			Resource:  resource,
			LabelName: labelValue,
		}
	}
	return labelValue, nil
}

func GetStringifiedYamlFromFilePath(filePath string) (string, error) {
	file, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	return string(file), err
}

// CalculateHash returns hash for interfaceToBeHashed.
func CalculateHash(interfaceToBeHashed any) (uint32, error) {
	data, err := json.Marshal(interfaceToBeHashed)
	if err != nil {
		return 0, err
	}

	h := fnv.New32a()
	h.Write(data)
	return h.Sum32(), nil
}

func GetCacheFunc(labelSelector labels.Set) cache.NewCacheFunc {
	return cache.BuilderWithOptions(
		cache.Options{
			SelectorsByObject: cache.SelectorsByObject{
				&v1.Secret{}: {
					Label: labels.SelectorFromSet(
						labelSelector,
					),
				},
			},
		},
	)
}

// JoinYAMLDocuments joins provided documents by replacing any leading/trailing markers and whitespaces
// with a single YAML marker between any two documents.
func JoinYAMLDocuments(yamlDocs [][]byte) string {
	if len(yamlDocs) == 0 {
		return ""
	}
	var res bytes.Buffer
	for i, ydoc := range yamlDocs {
		if len(ydoc) == 0 {
			continue
		}

		if i > 0 {
			res.Write([]byte("---\n"))
		}

		trimmed := bytes.TrimSpace(ydoc)                     // get rid of all the surrounding whitespaces
		trimmed = bytes.TrimPrefix(trimmed, []byte("---\n")) // get rid of the leading marker, if any
		trimmed = bytes.TrimSuffix(trimmed, []byte("---"))   // get rid of the trailing marker, if any
		trimmed = bytes.TrimSpace(trimmed)                   // get rid of any remaining surrounding whitespaces
		res.Write(append(trimmed, []byte("\n")...))          // ensure single newline at the end
	}
	return res.String()
}
