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
}

func (ts *FakeService) Clear() *FakeService {
	ts.registeredDescriptors = nil
	return ts
}

func (ts *FakeService) Register(descBytes []byte) *FakeService {
	registered := registeredDescriptor{
		rawDesc: descBytes,
	}
	ts.registeredDescriptors = append(ts.registeredDescriptors, registered)
	return ts
}

func (ts *FakeService) RegisterWithNameVersionOverride(name, version string, descBytes []byte) *FakeService {
	registered := registeredDescriptor{
		withOverride:        true,
		nameOverride:    name,
		versionOverride: version,
		rawDesc:         descBytes,
	}

	ts.registeredDescriptors = append(ts.registeredDescriptors, registered)
	return ts
}

type registeredDescriptor struct {
	withOverride        bool
	nameOverride    string
	versionOverride string
	rawDesc         []byte
}

func (s *FakeService) GetComponentDescriptor(ctx context.Context, ocmi ocmidentity.Component) (*types.Descriptor, error) {
	for _, rd := range s.registeredDescriptors {
		if rd.withOverride {
			if rd.nameOverride == ocmi.Name() && rd.versionOverride == ocmi.Version() {
				// user is asking for the descriptor that was registered with specific name and version override
				result, err := deserialize(rd.rawDesc, ocmi)
				if err != nil {
					return nil, err
				}
				result.Name = ocmi.Name()
				result.Version = ocmi.Version()
				return &types.Descriptor{
					ComponentDescriptor: result,
				}, nil
			}
		} else {
			// check if the registered descriptor matches the requested name and version
			result, err := deserialize(rd.rawDesc, ocmi)
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

	return nil, fmt.Errorf("component descriptor with name: %q and version %q not found", ocmi.Name(), ocmi.Version()) //nolint:err113 // no need for typed error
}
