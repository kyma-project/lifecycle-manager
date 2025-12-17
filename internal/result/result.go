package result

type UseCase string

type Result struct {
	UseCase UseCase
	Err     error
}
