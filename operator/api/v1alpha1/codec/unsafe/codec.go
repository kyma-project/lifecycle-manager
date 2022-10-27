package unsafe

import (
	"encoding/json"

	v2 "github.com/gardener/component-spec/bindings-go/apis/v2"
)

// DecodeV2 decodes a component into the given object.
// DecodeV2 is much simpler than the library decode to allow for much faster decoding
// it does not use reflection for obj analysis, by default does not allow validation and json schema checks
// also it does not make use of the kubernetes yaml package internal yaml to json conversion
// it is meant to be used inside controllers that already have validation running safely within their webhooks
// and can assume valid state of an object during reconciliation
// Because DecodeV2 does not need to introspect the metadata of the descriptor, it does not need to
// call an explicit Unmarshal on the metadata twice for Version Interpretation.
func DecodeV2(data []byte, obj *v2.ComponentDescriptor) error {
	if err := json.Unmarshal(data, obj); err != nil {
		return err
	}

	return v2.DefaultComponent(obj)
}

// EncodeV2 encodes a component or component list into the given object.
// EncodeV2 is much simpler than the library version to allow for much faster encoding.
func EncodeV2(obj *v2.ComponentDescriptor) ([]byte, error) {
	obj.Metadata.Version = v2.SchemaVersion
	if err := v2.DefaultComponent(obj); err != nil {
		return nil, err
	}
	return json.Marshal(obj)
}
