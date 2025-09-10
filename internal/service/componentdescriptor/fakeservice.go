package componentdescriptor

import (
	"context"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types/ocmidentity"
)

// Fake Service implementation for tests requiring component descriptors.
// It supports registering descriptors with name and version overrides - useful for scenarios
// where a single descriptor is used for multiple test cases with slightly different module names.
// Defining this fake here has the advantage of using the same internal
// deserialization logic as the real service, which makes this fake a bit more "real".
type FakeService struct {
	registeredDescriptors []registeredDescriptor
	stopped               bool
}

func NewFakeService(descBytes []byte) *FakeService {
	return (&FakeService{}).Register(descBytes)
}

func (s *FakeService) GetComponentDescriptor(ctx context.Context, ocmi ocmidentity.Component) (
	*types.Descriptor, error,
) {
	if s.stopped {
		panic("cannot get from a stopped FakeService")
	}

	for _, entry := range s.registeredDescriptors {
		if entry.withOverride {
			if entry.nameOverride == ocmi.Name() && entry.versionOverride == ocmi.Version() {
				// user is asking for the descriptor
				// that was registered with specific name and version override
				return s.deserializeOverride(entry, ocmi)
			}
		} else {
			// check if the registered descriptor matches the requested name and version
			result, err := deserialize(entry.rawDesc, ocmi)
			if err != nil {
				return nil, err
			}
			if result.Name == ocmi.Name() && result.Version == ocmi.Version() {
				return &types.Descriptor{
					ComponentDescriptor: result,
				}, nil
			}
		}
	}

	//nolint: err113 // it's only used in tests, there is no point in making it a public API error type
	notFoundErr := fmt.Errorf("component descriptor with name: %q and version %q not found",
		ocmi.Name(),
		ocmi.Version(),
	)

	return nil, notFoundErr
}

func (ts *FakeService) Clear() *FakeService {
	if ts.stopped {
		panic("cannot clear a stopped FakeService")
	}
	ts.registeredDescriptors = nil
	return ts
}

func (ts *FakeService) Register(descBytes []byte) *FakeService {
	if ts.stopped {
		panic("cannot register to a stopped FakeService")
	}
	registered := registeredDescriptor{
		rawDesc: descBytes,
	}
	ts.registeredDescriptors = append(ts.registeredDescriptors, registered)
	return ts
}

func (ts *FakeService) RegisterWithNameVersionOverride(name, version string, descBytes []byte) *FakeService {
	if ts.stopped {
		panic("cannot register to a stopped FakeService")
	}
	registered := registeredDescriptor{
		withOverride:    true,
		nameOverride:    name,
		versionOverride: version,
		rawDesc:         descBytes,
	}

	ts.registeredDescriptors = append(ts.registeredDescriptors, registered)
	return ts
}

func (fs *FakeService) Stop() {
	fs.stopped = true
}

func (fs *FakeService) Resume() bool {
	if fs.stopped {
		fs.stopped = false
		return true
	}
	return false
}

func (s *FakeService) deserializeOverride(entry registeredDescriptor, ocmi ocmidentity.Component) (
	*types.Descriptor, error,
) {
	result, err := deserialize(entry.rawDesc, ocmi)
	if err != nil {
		return nil, err
	}
	result.Name = ocmi.Name()
	result.Version = ocmi.Version()
	return &types.Descriptor{
		ComponentDescriptor: result,
	}, nil
}

type registeredDescriptor struct {
	withOverride    bool
	nameOverride    string
	versionOverride string
	rawDesc         []byte
}
