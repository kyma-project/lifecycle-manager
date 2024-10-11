package shared

import "strings"

const (
	KymaKind              Kind = "Kyma"
	ModuleTemplateKind    Kind = "ModuleTemplate"
	WatcherKind           Kind = "Watcher"
	ManifestKind          Kind = "Manifest"
	ModuleReleaseMetaKind Kind = "ModuleReleaseMeta"
)

type Kind string

func (k Kind) Plural() string {
	return strings.ToLower(string(k)) + "s"
}
