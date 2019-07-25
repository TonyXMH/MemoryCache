package memory_cache

import "errors"

var (
	ErrNotFound           = errors.New("Key not found in cache")
	ErrNotFoundOrLoadable = errors.New("Key not found and could not be loaded into cache")
)
