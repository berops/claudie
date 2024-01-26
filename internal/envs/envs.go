package envs

import (
	"os"
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
	//ContextBoxURL is a listening URL for ContextBox module
	ContextBoxURL = os.Getenv("CONTEXT_BOX_HOSTNAME") + ":" + os.Getenv("CONTEXT_BOX_PORT")
	//OperatorURL is a listening URL for claudie-operator connection
	OperatorURL = os.Getenv("OPERATOR_HOSTNAME") + ":" + os.Getenv("OPERATOR_PORT")
	//DatabaseURL is a listening URL for Database
	DatabaseURL = os.Getenv("DATABASE_URL")
	//KuberURL is a listening URL for Kuber module
	KuberURL = os.Getenv("KUBER_HOSTNAME") + ":" + os.Getenv("KUBER_PORT")
	//MinioURL is a listening URL for Minio deployment
	MinioURL = "http://" + os.Getenv("MINIO_HOSTNAME") + ":" + os.Getenv("MINIO_PORT")
	//ExternalS3Bucket is the name of an AWS S3 Bucket
	ExternalS3Bucket = os.Getenv("EXTERNAL_S3_BUCKET")
	//DynamoURL is a listening URL for DynamoDB local deployment
	DynamoURL = os.Getenv("DYNAMO_URL")
	//DynamoTable is the name of the DB table used for state locking
	DynamoTable = os.Getenv("DYNAMO_TABLE_NAME")
	// AwsAccesskeyId is part of credentials needed for connecting to dynamoDB
	AwsAccesskeyId = os.Getenv("AWS_ACCESS_KEY_ID")
	// AwsSecretAccessKey is part of credentials needed for connecting to dynamoDB
	AwsSecretAccessKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	// AwsRegion is part of credentials needed for connecting to dynamoDB
	AwsRegion = os.Getenv("AWS_REGION")
	//MinioAccessKey for backend
	MinioAccessKey = os.Getenv("MINIO_ROOT_USER")
	//MinioSecretKey for backend
	MinioSecretKey = os.Getenv("MINIO_ROOT_PASSWORD")
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

	if ContextBoxURL == ":" {
		ContextBoxURL = "localhost:50055"
	}
	ContextBoxURL = strings.ReplaceAll(ContextBoxURL, ":tcp://", "")

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

	if MinioURL == "http://:" {
		MinioURL = "http://localhost:9000"
	}
	if DynamoURL == "http://:" {
		DynamoURL = "http://localhost:8000"
	}
	if DynamoTable == "" {
		DynamoTable = "claudie"
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
	if MinioAccessKey == "" {
		MinioAccessKey = "minioadmin"
	}
	if MinioSecretKey == "" {
		MinioSecretKey = "minioadmin"
	}
	if LogLevel == "" {
		LogLevel = "info"
	}
}
