package envs

import (
	"os"
	"strconv"
	"strings"
)

var (
	// NatsClusterURL for connected to a nats cluster of [NatsClusterSize].
	NatsClusterURL = GetOrDefault("NATS_CLUSTER_URL", "nats://127.0.0.1") + ":" + GetOrDefault("NATS_CLUSTER_PORT", "4222")

	// NatsClusterSize for expected when connecting using the [NatsClusterURL]
	NatsClusterSize = GetOrDefaultInt("NATS_CLUSTER_SIZE", 1)

	// Name of the Jetstream when connected to the [NatsClusterURL] through which the communication
	// will be exchanged.
	NatsClusterJetstreamName = GetOrDefault("NATS_CLUSTER_JETSTREAM_NAME", "claudie-internal")

	// ManagerURL is a listening URL for Manager module
	ManagerURL = os.Getenv("MANAGER_HOSTNAME") + ":" + os.Getenv("MANAGER_PORT")

	// OperatorURL is a listening URL for claudie-operator connection
	OperatorURL = os.Getenv("OPERATOR_HOSTNAME") + ":" + os.Getenv("OPERATOR_PORT")

	// DatabaseURL is a listening URL for Database
	DatabaseURL = os.Getenv("DATABASE_URL")

	// BucketEndpoint is a listening URL for Minio deployment
	// If not defined it will use and external S3 Bucket,
	// by using AWS_REGION and BUCKET_NAME variables
	BucketEndpoint = os.Getenv("BUCKET_URL")

	// BucketName is the name of the bucket use for state
	// If not defined it will default to "claudie-tf-state"
	BucketName = os.Getenv("BUCKET_NAME")

	// AwsAccesskeyId is part of credentials needed for connecting to bucket
	AwsAccesskeyId = os.Getenv("AWS_ACCESS_KEY_ID")

	// AwsSecretAccessKey is part of credentials needed for connecting to bucket
	AwsSecretAccessKey = os.Getenv("AWS_SECRET_ACCESS_KEY")

	// AwsRegion is part of credentials needed for connecting to bucket
	AwsRegion = os.Getenv("AWS_REGION")

	// Namespace of current deployment
	// NOTE: namespace should be left empty if env var not been set
	Namespace = os.Getenv("NAMESPACE")

	// Golang log level
	LogLevel = os.Getenv("GOLANG_LOG")
)

// func init is used as setter for default values in case the env var has not been set
func init() {
	if ManagerURL == ":" {
		ManagerURL = "localhost:50055"
	}
	ManagerURL = strings.ReplaceAll(ManagerURL, ":tcp://", "")

	if OperatorURL == ":" {
		OperatorURL = "localhost:50058"
	}
	OperatorURL = strings.ReplaceAll(OperatorURL, ":tcp://", "")

	if DatabaseURL == "" {
		DatabaseURL = "mongodb://localhost:27017"
	}

	if BucketName == "" {
		BucketName = "claudie-tf-state-files"
	}

	if AwsAccesskeyId == "" {
		AwsAccesskeyId = "fake"
	}
	if AwsSecretAccessKey == "" {
		AwsSecretAccessKey = "fake"
	}
	if AwsRegion == "" {
		AwsRegion = "local"
	}
	if LogLevel == "" {
		LogLevel = "info"
	}
}

// GetOrDefault take a string representing environment variable as an argument, and a default value
// If the environment variable is not defined, it returns the provided default value.
func GetOrDefault(envKey string, defaultVal string) string {
	v, present := os.LookupEnv(envKey)
	if present {
		return v
	} else {
		return defaultVal
	}
}

// GetOrDefaultInt retrieves the environment variable and parses it as an Integer. On any error
// or if the environment variable does not exists, the default value is returned.
func GetOrDefaultInt(key string, def int) int {
	v, present := os.LookupEnv(key)
	if present {
		v, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return def
		}
		return int(v)
	}
	return def
}
