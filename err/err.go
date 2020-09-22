package err

import "errors"

var (
	AlreadyInQueue        = errors.New("ALREADY_IN_QUEUE")
	EnvDoesNotExist       = errors.New("ENV_DOES_NOT_EXIST")
	InvalidResourceFormat = errors.New("INVALID_RESOURCE_FORMAT")
	NoResourceProvided    = errors.New("NO_RESOURCE_PROVIDED")
	NotInQueue            = errors.New("NOT_IN_QUEUE")
	ResourceDoesNotExist  = errors.New("RESOURCE_DOES_NOT_EXIST")
)
