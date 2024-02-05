package outboundAdapters

import (
	"errors"

	"github.com/berops/claudie/internal/envs"
)

var (
	awsRegion          = envs.AwsRegion
	awsAccessKeyId     = envs.AwsAccesskeyId
	awsSecretAccessKey = envs.AwsSecretAccessKey
)

var (
	// ErrKeyNotExists is returned when the key is not present in the object storage.
	ErrKeyNotExists = errors.New("key is not present in bucket")
)
