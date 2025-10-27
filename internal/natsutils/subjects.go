package natsutils

// utility constants around replying for received NATs messages.
const (
	// Nats Header to set, so that services can check, if set
	// and send the responses to the approriate subjects.
	ReplyToHeader = "claudie-internal-reply-to"

	// If the [ReplyToHeader] is not set, the reply can be interpreted
	// as [ReplyDiscard]
	ReplyDiscard = ""
)

// A list of default claudie related NATS subjects.
const (
	// Subject related to Ansibler service only request Messages.
	AnsiblerRequests = "claudie-internal-cluster-requests-ansibler"

	// Subject related to Ansibler service only response Messages.
	AnsiblerResponse = "claudie-internal-cluster-response-ansibler"

	// Subject related to Kuber service only request Messages.
	KuberRequests = "claudie-internal-cluster-requests-kuber"

	// Subject related to Kuber service only response Messages.
	KuberResponse = "claudie-internal-cluster-response-kuber"

	// Subject related to KubeEleven service only request Messages.
	KubeElevenRequest = "claudie-internal-cluster-request-kube-eleven"

	// Subject related to KubeEleven service only response Messages.
	KubeElevenResponse = "claudie-internal-cluster-response-kube-eleven"

	// Subject related to Terraformer service only request Messages.
	TerraformerRequest = "claudie-internal-cluster-request-terraformer"

	// Subject related to Terraformer service only response Messages.
	TerraformerResponse = "claudie-internal-cluster-response-terraformer"

	// Misc is a subject unrelated to any of the other above subjects.
	Misc = "claudie-internal-misc"
)

// Default subjects that are used if no are supplied in the [Client.JetStreamWorkQueue] func.
var DefaultSubjects = [...]string{
	AnsiblerRequests,
	AnsiblerResponse,
	KuberRequests,
	KuberResponse,
	KubeElevenRequest,
	KubeElevenResponse,
	TerraformerRequest,
	TerraformerResponse,
	Misc,
}
