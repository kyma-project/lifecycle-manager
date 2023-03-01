package v1beta1

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	"github.com/kyma-project/lifecycle-manager/internal"

	"github.com/google/go-containerregistry/pkg/crane"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/google/go-containerregistry/pkg/authn"
	"regexp"
	yaml2 "sigs.k8s.io/yaml"
)

func GetPathFromExtractedTarGz(ctx context.Context,
	imageSpec v1beta1.ImageSpec,
	keyChain authn.Keychain,
) (string, error) {
	imageRef := fmt.Sprintf("%s/%s@%s", imageSpec.Repo, imageSpec.Name, imageSpec.Ref)

	// check existing dir
	// if dir exists return existing dir
	installPath := GetFsChartPath(imageSpec)
	dir, err := os.Open(installPath)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("opening dir for installs caused an error %s: %w", imageRef, err)
	}
	if dir != nil {
		return installPath, nil
	}

	// pull image layer
	layer, err := pullLayer(ctx, imageRef, keyChain)
	if err != nil {
		return "", err
	}

	// uncompress chart to install path
	blobReadCloser, err := layer.Compressed()
	if err != nil {
		return "", fmt.Errorf("fetching blob for compressed layer %s: %w", imageRef, err)
	}

	uncompressedStream, err := gzip.NewReader(blobReadCloser)
	if err != nil {
		return "", fmt.Errorf("failure in NewReader() while extracting TarGz %s: %w", imageRef, err)
	}
	tarReader := tar.NewReader(uncompressedStream)
	return installPath, writeTarGzContent(installPath, tarReader, imageRef)
}

func writeTarGzContent(installPath string, tarReader *tar.Reader, layerReference string) error {
	// create dir for uncompressed chart
	if err := os.MkdirAll(installPath, fs.ModePerm); err != nil {
		return fmt.Errorf(
			"failure in MkdirAll() while extracting TarGz for installPath %s: %w",
			layerReference, err,
		)
	}

	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("failed Next() while extracting TarGz %s: %w", layerReference, err)
		}

		destDir, destFile := path.Split(header.Name)
		destinationPath, err := internal.CleanFilePathJoin(installPath, destDir)
		if err != nil {
			return err
		}

		if err := os.MkdirAll(destinationPath, fs.ModePerm); err != nil {
			return fmt.Errorf(
				"failure in MkdirAll() while extracting TarGz for destinationPath %s: %w",
				layerReference, err,
			)
		}
		if err = handleExtractedHeaderFile(header, tarReader, destFile, destinationPath, layerReference); err != nil {
			return err
		}
	}
	return nil
}

var ErrUnknownTypeDuringHeaderExtraction = errors.New("unknown type encountered during header extraction")

func handleExtractedHeaderFile(
	header *tar.Header,
	reader io.Reader,
	file, destinationPath, layerReference string,
) error {
	switch header.Typeflag {
	case tar.TypeDir:
		if err := os.MkdirAll(destinationPath, internal.OthersReadExecuteFilePermission); err != nil {
			return fmt.Errorf("failure in Mkdir() storage while extracting TarGz %s: %w", layerReference, err)
		}
	case tar.TypeReg:
		filePath := path.Join(destinationPath, file)
		//nolint:nosnakecase
		outFile, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
		if err != nil {
			return fmt.Errorf("file create failed while extracting TarGz %s: %w", layerReference, err)
		}
		if _, err := io.Copy(outFile, reader); err != nil {
			return fmt.Errorf("file copy storage failed while extracting TarGz %s: %w", layerReference, err)
		}
		return outFile.Close()
	default:
		return fmt.Errorf(
			"error while extracting TarGz %v in %s: %w",
			header.Typeflag, destinationPath, ErrUnknownTypeDuringHeaderExtraction,
		)
	}
	return nil
}

func DecodeUncompressedYAMLLayer(ctx context.Context,
	imageSpec v1beta1.ImageSpec,
	keyChain authn.Keychain,
) (interface{}, error) {
	configFilePath := GetConfigFilePath(imageSpec)

	imageRef := fmt.Sprintf("%s/%s@%s", imageSpec.Repo, imageSpec.Name, imageSpec.Ref)
	// check existing file
	decodedFile, err := internal.GetYamlFileContent(configFilePath)
	if err == nil {
		return decodedFile, nil
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("opening file for install imageSpec caused an error %s: %w", imageRef, err)
	}

	// proceed only if file was not found
	// yaml is not compressed
	layer, err := pullLayer(ctx, imageRef, keyChain)
	if err != nil {
		return nil, err
	}
	blob, err := layer.Uncompressed()
	if err != nil {
		return nil, fmt.Errorf("fetching blob for uncompressed layer %s: %w", imageRef, err)
	}

	return writeYamlContent(blob, imageRef, configFilePath)
}

func pullLayer(ctx context.Context, imageRef string, keyChain authn.Keychain) (v1.Layer, error) {
	noSchemeImageRef := noSchemeURL(imageRef)
	isInsecureLayer, _ := regexp.MatchString("^http://", imageRef)
	if isInsecureLayer {
		return crane.PullLayer(noSchemeImageRef, crane.Insecure, crane.WithAuthFromKeychain(keyChain))
	}
	return crane.PullLayer(noSchemeImageRef, crane.WithAuthFromKeychain(keyChain), crane.WithContext(ctx))
}

func noSchemeURL(url string) string {
	regex := regexp.MustCompile(`^https?://`)
	return regex.ReplaceAllString(url, "")
}

func writeYamlContent(blob io.ReadCloser, layerReference string, filePath string) (interface{}, error) {
	var decodedConfig interface{}
	err := yaml.NewYAMLOrJSONDecoder(blob, internal.YamlDecodeBufferSize).Decode(&decodedConfig)
	if err != nil {
		return nil, fmt.Errorf("yaml blob decoding resulted in an error %s: %w", layerReference, err)
	}

	bytes, err := yaml2.Marshal(decodedConfig)
	if err != nil {
		return nil, fmt.Errorf("yaml marshal for install config caused an error %s: %w", layerReference, err)
	}

	// close file
	return decodedConfig, internal.WriteToFile(filePath, bytes)
}
