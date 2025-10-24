package componentdescriptor

import (
	"os"

	"github.com/go-logr/logr"

	"github.com/kyma-project/lifecycle-manager/internal/service/componentdescriptor"
)

// ComposeComponentDescriptorService manges creation of a new instance of the ComponentDescriptor Service.
func ComposeComponentDescriptorService(
	repository componentdescriptor.OCIRepository,
	logger logr.Logger,
	bootstrapFailedExitCode int,
) *componentdescriptor.Service {
	tarExtractor := componentdescriptor.NewTarExtractor()
	fileExtractor := componentdescriptor.NewFileExtractor(tarExtractor)

	service, err := componentdescriptor.NewService(repository, fileExtractor)
	if err != nil {
		logger.Error(err, "failed to create OCM descriptor service")
		os.Exit(bootstrapFailedExitCode)
	}

	return service
}
