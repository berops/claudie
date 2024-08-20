package utils

type BuilderTopic string

const (
	//TopicDeleteInputManifest BuilderTopic = "delete-input-manifest"

	//TopicDeleteK8sCluster BuilderTopic = "delete-k8s-cluster"

	//TopicApplyWorkflow BuilderTopic = "apply-workflow"

	//TopicApplyIR BuilderTopic = "apply-ir"

	//TopicApplyEndpointReplace BuilderTopic = "apply-endpoint-replace"

	//TopicDeleteNodes BuilderTopic = "delete-k8s-nodes"

	// TopicDeleteLoadBalancers BuilderTopic = "delete-load-balancers"

	TopicCreateWorkflow BuilderTopic = "create-workflow"
	TopicUpdateWorkflow BuilderTopic = "update-workflow"
	TopicDeleteWorkflow BuilderTopic = "delete-workflow"
)
