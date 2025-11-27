package envs

import (
	"os"
	"strconv"
	"strings"
)

// Hostnames and ports on what services are listening
var (
	//TerraformerURL is a listening URL for Terraformer module
	TerraformerURL = os.Getenv("TERRAFORMER_HOSTNAME") + ":" + os.Getenv("TERRAFORMER_PORT")
	//AnsiblerURL is a listening URL for Ansibler module
	AnsiblerURL = os.Getenv("ANSIBLER_HOSTNAME") + ":" + os.Getenv("ANSIBLER_PORT")
	//KubeElevenURL is a listening URL for KubeEleven module
	KubeElevenURL = os.Getenv("KUBE_ELEVEN_HOSTNAME") + ":" + os.Getenv("KUBE_ELEVEN_PORT")
	//ManagerURL is a listening URL for Manager module
	ManagerURL = os.Getenv("MANAGER_HOSTNAME") + ":" + os.Getenv("MANAGER_PORT")
	//OperatorURL is a listening URL for claudie-operator connection
	OperatorURL = os.Getenv("OPERATOR_HOSTNAME") + ":" + os.Getenv("OPERATOR_PORT")
	//DatabaseURL is a listening URL for Database
	DatabaseURL = os.Getenv("DATABASE_URL")
	//KuberURL is a listening URL for Kuber module
	KuberURL = os.Getenv("KUBER_HOSTNAME") + ":" + os.Getenv("KUBER_PORT")
	//BucketEndpoint is a listening URL for Minio deployment
	//If not defined it will use and external S3 Bucket,
	//by using AWS_REGION and BUCKET_NAME variables
	BucketEndpoint = os.Getenv("BUCKET_URL")
	//BucketName is the name of the bucket use for state
	//If not defined it will default to "claudie-tf-state"
	BucketName = os.Getenv("BUCKET_NAME")
	// AwsAccesskeyId is part of credentials needed for connecting to bucket
	AwsAccesskeyId = os.Getenv("AWS_ACCESS_KEY_ID")
	// AwsSecretAccessKey is part of credentials needed for connecting to bucket
	AwsSecretAccessKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	// AwsRegion is part of credentials needed for connecting to bucket
	AwsRegion = os.Getenv("AWS_REGION")

	//Namespace of current deployment
	//NOTE: namespace should be left empty if env var not been set
	Namespace = os.Getenv("NAMESPACE")
	//Golang log level
	LogLevel = os.Getenv("GOLANG_LOG")
)

// func init is used as setter for default values in case the env var has not been set
func init() {
	if TerraformerURL == ":" {
		TerraformerURL = "localhost:50052"
	}
	TerraformerURL = strings.ReplaceAll(TerraformerURL, ":tcp://", "")

	if AnsiblerURL == ":" {
		AnsiblerURL = "localhost:50053"
	}
	AnsiblerURL = strings.ReplaceAll(AnsiblerURL, ":tcp://", "")

	if KubeElevenURL == ":" {
		KubeElevenURL = "localhost:50054"
	}
	KubeElevenURL = strings.ReplaceAll(KubeElevenURL, ":tcp://", "")

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

	if KuberURL == ":" {
		KuberURL = "localhost:50057"
	}
	KuberURL = strings.ReplaceAll(KuberURL, ":tcp://", "")

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
