package v1beta1

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
)

const (
	configFileName = "installConfig.yaml"
	configsFolder  = "configs"
)

func GetFsChartPath(imageSpec v1beta1.ImageSpec) string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("%s-%s", imageSpec.Name, imageSpec.Ref))
}

func GetConfigFilePath(config v1beta1.ImageSpec) string {
	return filepath.Join(os.TempDir(), configsFolder, config.Ref, configFileName)
}
