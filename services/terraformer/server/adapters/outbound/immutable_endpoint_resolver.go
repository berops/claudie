package outboundAdapters

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go/endpoints"
	"net/url"
)

type immutableResolver struct{ endpoint string }

func (i *immutableResolver) ResolveEndpoint(_ context.Context, params s3.EndpointParameters) (transport.Endpoint, error) {
	u, err := url.Parse(i.endpoint)
	if err != nil {
		return transport.Endpoint{}, err
	}

	u.Path += "/" + *params.Bucket
	return transport.Endpoint{URI: *u}, nil
}
