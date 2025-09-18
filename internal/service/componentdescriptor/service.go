package componentdescriptor

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types/ocmidentity"
	"io"
	"ocm.software/ocm/api/ocm/compdesc"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"

	"ocm.software/ocm/api/ocm/extensions/repositories/genericocireg"
)

const (
	ComponentDescriptorFileName = "component-descriptor.yaml"
)

var (
	ErrInvalidArg = errors.New("invalid argument")
)

type OCIRepository interface {
	GetConfigFile(ctx context.Context, name, tag string) ([]byte, error)
	PullLayer(ctx context.Context, name, tag, digest string) (v1.Layer, error)
	HostRef() string //example result: "k3d-kcp-registry.localhost:5000"
}

// Helper interface to simplify testing
type FileExtractor interface {
	ExtractFile(layer v1.Layer, fileName string) ([]byte, error)
}

type Service struct {
	ociRepository OCIRepository
	fileExtractor FileExtractor
}

func NewService(ociRepository OCIRepository) (*Service, error) {
	if ociRepository == nil {
		return nil, fmt.Errorf("ociRepository must not be nil: %w", ErrInvalidArg)
	}

	return &Service{
		ociRepository: ociRepository,
		fileExtractor: &defaultFileExtractor{},
	}, nil
}

func (s *Service) GetComponentDescriptor(ctx context.Context, ocmi ocmidentity.Component) (*types.Descriptor, error) {

	//{"componentDescriptorLayer":{"mediaType":"application/vnd.ocm.software.component-descriptor.v2+yaml+tar","digest":"sha256:4e51d8f80b88bdbd208e6e22314376a0d5212026bf3054f8ef79d43250e5182b","size":4608}}
	//ref := fmt.Sprintf("k3d-kcp-registry.localhost:5000/component-descriptors/%s:%s", name, version)

	commonErrMsg := func() string {
		return fmt.Sprintf("repository=%q, name=%q and version=%q",
			s.ociRepository.HostRef(), ocmi.Name(), ocmi.Version())
	}

	// Fetch the image config to get the ComponentDescriptor layer info
	configBytes, err := s.ociRepository.GetConfigFile(ctx, ocmi.Name(), ocmi.Version())
	if err != nil {
		return nil, fmt.Errorf("failed to get config file for %s: %w", commonErrMsg(), err)
	}

	ocmArtifactConfig := genericocireg.ComponentDescriptorConfig{}

	err = json.Unmarshal(configBytes, &ocmArtifactConfig)
	if err != nil {
		return nil,
			fmt.Errorf("failed to unmarshal config data into ComponentDescriptorConfig for %s: %w",
				commonErrMsg(), err)
	}

	if ocmArtifactConfig.ComponentDescriptorLayer == nil {
		return nil, fmt.Errorf("ComponentDescriptorLayer not found in ComponentDescriptorConfig for %s: %w",
			commonErrMsg(), err)
	}

	compDescLayerDigest := ocmArtifactConfig.ComponentDescriptorLayer.Digest
	if string(compDescLayerDigest) == "" {
		return nil,
			fmt.Errorf("ComponentDescriptorLayer.Digest is empty in ComponentDescriptorConfig for %s: %w",
				commonErrMsg(), err)
	}

	layer, err := s.ociRepository.PullLayer(ctx, ocmi.Name(), ocmi.Version(), string(compDescLayerDigest))
	//layer, err := crane.PullLayer(fmt.Sprintf("%s@%s", ref, digest), crane.Insecure)
	if err != nil {
		return nil, fmt.Errorf("failed to pull layer for %s with digest=%q: %w",
			commonErrMsg(), string(compDescLayerDigest), err)
	}

	compdescBytes, err := s.fileExtractor.ExtractFile(layer, ComponentDescriptorFileName)
	if err != nil {
		return nil,
			fmt.Errorf("failed to extract component descriptor from layer fetched from %s with digest=%q: %w",
				commonErrMsg(), string(compDescLayerDigest), err)
	}

	cd, err := compdesc.Decode(compdescBytes)
	if err != nil {
		fmt.Println(strings.Repeat("=", 80))
		fmt.Println(string(compdescBytes))
		fmt.Println(strings.Repeat("=", 80))
		return nil, fmt.Errorf("failed to decode component descriptor fetched from %s: %w",
			commonErrMsg(), err)
	}

	return &types.Descriptor{
		ComponentDescriptor: cd,
	}, nil
}

type defaultFileExtractor struct {
}

func (d *defaultFileExtractor) ExtractFile(layer v1.Layer, fileName string) ([]byte, error) {

	layerReader, err := layer.Uncompressed()
	if err != nil {
		digest, derr := layer.Digest()
		if derr != nil {
			err = errors.Join(err, derr)
		}
		return nil,
			fmt.Errorf("failed to get uncompressed reader for layer with digest=%q: %w", digest, err)
	}
	defer layerReader.Close()

	layerBytes, err := io.ReadAll(layerReader)
	if err != nil {
		digest, derr := layer.Digest()
		if derr != nil {
			err = errors.Join(err, derr)
		}
		return nil,
			fmt.Errorf("failed to read layer contents for layer with digest=%q: %w", digest, err)
	}

	compdescBytes, err := unTar(layerBytes, fileName)
	if err != nil {
		digest, derr := layer.Digest()
		if derr != nil {
			err = errors.Join(err, derr)
		}
		return nil,
			fmt.Errorf("failed to extract data from TAR archive for file=%q in layer with digest=%q: %w",
				fileName, digest, err)
	}

	return compdescBytes, nil
}

// unTar extracts the file with expectedName from the given tarBytes and returns its content.
func unTar(tarBytes []byte, expectedName string) ([]byte, error) {
	tr := tar.NewReader(bytes.NewReader(tarBytes))

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break // end of archive
		}
		if err != nil {
			return nil, err
		}

		if hdr.Name == expectedName {
			var buf bytes.Buffer
			if _, err := io.Copy(&buf, tr); err != nil {
				return nil, err
			}
			return buf.Bytes(), nil
		}
	}

	return nil, fmt.Errorf("%s not found in TAR archive", expectedName)
}
