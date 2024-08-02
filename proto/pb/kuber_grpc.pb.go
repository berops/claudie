// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v5.27.1
// source: kuber.proto

package pb

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// KuberServiceClient is the client API for KuberService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type KuberServiceClient interface {
	// RemoveLBScrapeConfig removes scrape config for every LB detached from this
	// cluster.
	RemoveLBScrapeConfig(ctx context.Context, in *RemoveLBScrapeConfigRequest, opts ...grpc.CallOption) (*RemoveLBScrapeConfigResponse, error)
	// StoreLBScrapeConfig stores scrape config for every LB attached to this
	// cluster.
	StoreLBScrapeConfig(ctx context.Context, in *StoreLBScrapeConfigRequest, opts ...grpc.CallOption) (*StoreLBScrapeConfigResponse, error)
	// StoreClusterMetadata creates a secret, which holds the private key and a
	// list of public IP addresses of the cluster supplied.
	StoreClusterMetadata(ctx context.Context, in *StoreClusterMetadataRequest, opts ...grpc.CallOption) (*StoreClusterMetadataResponse, error)
	// DeleteClusterMetadata deletes the secret holding the private key and public
	// IP addresses of the cluster supplied.
	DeleteClusterMetadata(ctx context.Context, in *DeleteClusterMetadataRequest, opts ...grpc.CallOption) (*DeleteClusterMetadataResponse, error)
	// SetUpStorage installs Longhorn into the cluster.
	SetUpStorage(ctx context.Context, in *SetUpStorageRequest, opts ...grpc.CallOption) (*SetUpStorageResponse, error)
	// StoreKubeconfig creates a secret, which holds the kubeconfig of a
	// Claudie-created cluster.
	StoreKubeconfig(ctx context.Context, in *StoreKubeconfigRequest, opts ...grpc.CallOption) (*StoreKubeconfigResponse, error)
	// DeleteKubeconfig removes the secret that holds the kubeconfig of a
	// Claudie-created cluster.
	DeleteKubeconfig(ctx context.Context, in *DeleteKubeconfigRequest, opts ...grpc.CallOption) (*DeleteKubeconfigResponse, error)
	// DeleteNodes deletes the specified nodes from a k8s cluster.
	DeleteNodes(ctx context.Context, in *DeleteNodesRequest, opts ...grpc.CallOption) (*DeleteNodesResponse, error)
	// PatchNodes applies attributes like providerID, labels or taints to the
	// nodes.
	PatchNodes(ctx context.Context, in *PatchNodesRequest, opts ...grpc.CallOption) (*PatchNodesResponse, error)
	// SetUpClusterAutoscaler deploys Cluster Autoscaler and Autoscaler Adapter
	// for every cluster specified.
	SetUpClusterAutoscaler(ctx context.Context, in *SetUpClusterAutoscalerRequest, opts ...grpc.CallOption) (*SetUpClusterAutoscalerResponse, error)
	// DestroyClusterAutoscaler deletes Cluster Autoscaler and Autoscaler Adapter
	// for every cluster specified.
	DestroyClusterAutoscaler(ctx context.Context, in *DestroyClusterAutoscalerRequest, opts ...grpc.CallOption) (*DestroyClusterAutoscalerResponse, error)
	// PatchClusterInfoConfigMap updates the cluster-info config map in the
	// kube-public namespace with the the kubeconfig. This needs to be done after
	// an api endpoint change as the config map in the kube-public namespace is
	// used by kubeadm when joining.
	PatchClusterInfoConfigMap(ctx context.Context, in *PatchClusterInfoConfigMapRequest, opts ...grpc.CallOption) (*PatchClusterInfoConfigMapResponse, error)
	// PatchCubeProxyConfigMap updates the kube-proxy config map with the value of
	// the server: from the passed in kubeconfig and subsequently restarts all
	// kube-proxy pods in the kube-system namespace. This change needs to be done
	// after a endpoint change is performed.
	PatchKubeProxyConfigMap(ctx context.Context, in *PatchKubeProxyConfigMapRequest, opts ...grpc.CallOption) (*PatchKubeProxyConfigMapResponse, error)
	// CiliumRolloutRestart performs a rollout restart of the cilium daemonset.
	CiliumRolloutRestart(ctx context.Context, in *CiliumRolloutRestartRequest, opts ...grpc.CallOption) (*CiliumRolloutRestartResponse, error)
}

type kuberServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewKuberServiceClient(cc grpc.ClientConnInterface) KuberServiceClient {
	return &kuberServiceClient{cc}
}

func (c *kuberServiceClient) RemoveLBScrapeConfig(ctx context.Context, in *RemoveLBScrapeConfigRequest, opts ...grpc.CallOption) (*RemoveLBScrapeConfigResponse, error) {
	out := new(RemoveLBScrapeConfigResponse)
	err := c.cc.Invoke(ctx, "/claudie.KuberService/RemoveLBScrapeConfig", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *kuberServiceClient) StoreLBScrapeConfig(ctx context.Context, in *StoreLBScrapeConfigRequest, opts ...grpc.CallOption) (*StoreLBScrapeConfigResponse, error) {
	out := new(StoreLBScrapeConfigResponse)
	err := c.cc.Invoke(ctx, "/claudie.KuberService/StoreLBScrapeConfig", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *kuberServiceClient) StoreClusterMetadata(ctx context.Context, in *StoreClusterMetadataRequest, opts ...grpc.CallOption) (*StoreClusterMetadataResponse, error) {
	out := new(StoreClusterMetadataResponse)
	err := c.cc.Invoke(ctx, "/claudie.KuberService/StoreClusterMetadata", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *kuberServiceClient) DeleteClusterMetadata(ctx context.Context, in *DeleteClusterMetadataRequest, opts ...grpc.CallOption) (*DeleteClusterMetadataResponse, error) {
	out := new(DeleteClusterMetadataResponse)
	err := c.cc.Invoke(ctx, "/claudie.KuberService/DeleteClusterMetadata", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *kuberServiceClient) SetUpStorage(ctx context.Context, in *SetUpStorageRequest, opts ...grpc.CallOption) (*SetUpStorageResponse, error) {
	out := new(SetUpStorageResponse)
	err := c.cc.Invoke(ctx, "/claudie.KuberService/SetUpStorage", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *kuberServiceClient) StoreKubeconfig(ctx context.Context, in *StoreKubeconfigRequest, opts ...grpc.CallOption) (*StoreKubeconfigResponse, error) {
	out := new(StoreKubeconfigResponse)
	err := c.cc.Invoke(ctx, "/claudie.KuberService/StoreKubeconfig", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *kuberServiceClient) DeleteKubeconfig(ctx context.Context, in *DeleteKubeconfigRequest, opts ...grpc.CallOption) (*DeleteKubeconfigResponse, error) {
	out := new(DeleteKubeconfigResponse)
	err := c.cc.Invoke(ctx, "/claudie.KuberService/DeleteKubeconfig", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *kuberServiceClient) DeleteNodes(ctx context.Context, in *DeleteNodesRequest, opts ...grpc.CallOption) (*DeleteNodesResponse, error) {
	out := new(DeleteNodesResponse)
	err := c.cc.Invoke(ctx, "/claudie.KuberService/DeleteNodes", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *kuberServiceClient) PatchNodes(ctx context.Context, in *PatchNodesRequest, opts ...grpc.CallOption) (*PatchNodesResponse, error) {
	out := new(PatchNodesResponse)
	err := c.cc.Invoke(ctx, "/claudie.KuberService/PatchNodes", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *kuberServiceClient) SetUpClusterAutoscaler(ctx context.Context, in *SetUpClusterAutoscalerRequest, opts ...grpc.CallOption) (*SetUpClusterAutoscalerResponse, error) {
	out := new(SetUpClusterAutoscalerResponse)
	err := c.cc.Invoke(ctx, "/claudie.KuberService/SetUpClusterAutoscaler", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *kuberServiceClient) DestroyClusterAutoscaler(ctx context.Context, in *DestroyClusterAutoscalerRequest, opts ...grpc.CallOption) (*DestroyClusterAutoscalerResponse, error) {
	out := new(DestroyClusterAutoscalerResponse)
	err := c.cc.Invoke(ctx, "/claudie.KuberService/DestroyClusterAutoscaler", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *kuberServiceClient) PatchClusterInfoConfigMap(ctx context.Context, in *PatchClusterInfoConfigMapRequest, opts ...grpc.CallOption) (*PatchClusterInfoConfigMapResponse, error) {
	out := new(PatchClusterInfoConfigMapResponse)
	err := c.cc.Invoke(ctx, "/claudie.KuberService/PatchClusterInfoConfigMap", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *kuberServiceClient) PatchKubeProxyConfigMap(ctx context.Context, in *PatchKubeProxyConfigMapRequest, opts ...grpc.CallOption) (*PatchKubeProxyConfigMapResponse, error) {
	out := new(PatchKubeProxyConfigMapResponse)
	err := c.cc.Invoke(ctx, "/claudie.KuberService/PatchKubeProxyConfigMap", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *kuberServiceClient) CiliumRolloutRestart(ctx context.Context, in *CiliumRolloutRestartRequest, opts ...grpc.CallOption) (*CiliumRolloutRestartResponse, error) {
	out := new(CiliumRolloutRestartResponse)
	err := c.cc.Invoke(ctx, "/claudie.KuberService/CiliumRolloutRestart", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// KuberServiceServer is the server API for KuberService service.
// All implementations must embed UnimplementedKuberServiceServer
// for forward compatibility
type KuberServiceServer interface {
	// RemoveLBScrapeConfig removes scrape config for every LB detached from this
	// cluster.
	RemoveLBScrapeConfig(context.Context, *RemoveLBScrapeConfigRequest) (*RemoveLBScrapeConfigResponse, error)
	// StoreLBScrapeConfig stores scrape config for every LB attached to this
	// cluster.
	StoreLBScrapeConfig(context.Context, *StoreLBScrapeConfigRequest) (*StoreLBScrapeConfigResponse, error)
	// StoreClusterMetadata creates a secret, which holds the private key and a
	// list of public IP addresses of the cluster supplied.
	StoreClusterMetadata(context.Context, *StoreClusterMetadataRequest) (*StoreClusterMetadataResponse, error)
	// DeleteClusterMetadata deletes the secret holding the private key and public
	// IP addresses of the cluster supplied.
	DeleteClusterMetadata(context.Context, *DeleteClusterMetadataRequest) (*DeleteClusterMetadataResponse, error)
	// SetUpStorage installs Longhorn into the cluster.
	SetUpStorage(context.Context, *SetUpStorageRequest) (*SetUpStorageResponse, error)
	// StoreKubeconfig creates a secret, which holds the kubeconfig of a
	// Claudie-created cluster.
	StoreKubeconfig(context.Context, *StoreKubeconfigRequest) (*StoreKubeconfigResponse, error)
	// DeleteKubeconfig removes the secret that holds the kubeconfig of a
	// Claudie-created cluster.
	DeleteKubeconfig(context.Context, *DeleteKubeconfigRequest) (*DeleteKubeconfigResponse, error)
	// DeleteNodes deletes the specified nodes from a k8s cluster.
	DeleteNodes(context.Context, *DeleteNodesRequest) (*DeleteNodesResponse, error)
	// PatchNodes applies attributes like providerID, labels or taints to the
	// nodes.
	PatchNodes(context.Context, *PatchNodesRequest) (*PatchNodesResponse, error)
	// SetUpClusterAutoscaler deploys Cluster Autoscaler and Autoscaler Adapter
	// for every cluster specified.
	SetUpClusterAutoscaler(context.Context, *SetUpClusterAutoscalerRequest) (*SetUpClusterAutoscalerResponse, error)
	// DestroyClusterAutoscaler deletes Cluster Autoscaler and Autoscaler Adapter
	// for every cluster specified.
	DestroyClusterAutoscaler(context.Context, *DestroyClusterAutoscalerRequest) (*DestroyClusterAutoscalerResponse, error)
	// PatchClusterInfoConfigMap updates the cluster-info config map in the
	// kube-public namespace with the the kubeconfig. This needs to be done after
	// an api endpoint change as the config map in the kube-public namespace is
	// used by kubeadm when joining.
	PatchClusterInfoConfigMap(context.Context, *PatchClusterInfoConfigMapRequest) (*PatchClusterInfoConfigMapResponse, error)
	// PatchCubeProxyConfigMap updates the kube-proxy config map with the value of
	// the server: from the passed in kubeconfig and subsequently restarts all
	// kube-proxy pods in the kube-system namespace. This change needs to be done
	// after a endpoint change is performed.
	PatchKubeProxyConfigMap(context.Context, *PatchKubeProxyConfigMapRequest) (*PatchKubeProxyConfigMapResponse, error)
	// CiliumRolloutRestart performs a rollout restart of the cilium daemonset.
	CiliumRolloutRestart(context.Context, *CiliumRolloutRestartRequest) (*CiliumRolloutRestartResponse, error)
	mustEmbedUnimplementedKuberServiceServer()
}

// UnimplementedKuberServiceServer must be embedded to have forward compatible implementations.
type UnimplementedKuberServiceServer struct {
}

func (UnimplementedKuberServiceServer) RemoveLBScrapeConfig(context.Context, *RemoveLBScrapeConfigRequest) (*RemoveLBScrapeConfigResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RemoveLBScrapeConfig not implemented")
}
func (UnimplementedKuberServiceServer) StoreLBScrapeConfig(context.Context, *StoreLBScrapeConfigRequest) (*StoreLBScrapeConfigResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method StoreLBScrapeConfig not implemented")
}
func (UnimplementedKuberServiceServer) StoreClusterMetadata(context.Context, *StoreClusterMetadataRequest) (*StoreClusterMetadataResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method StoreClusterMetadata not implemented")
}
func (UnimplementedKuberServiceServer) DeleteClusterMetadata(context.Context, *DeleteClusterMetadataRequest) (*DeleteClusterMetadataResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteClusterMetadata not implemented")
}
func (UnimplementedKuberServiceServer) SetUpStorage(context.Context, *SetUpStorageRequest) (*SetUpStorageResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetUpStorage not implemented")
}
func (UnimplementedKuberServiceServer) StoreKubeconfig(context.Context, *StoreKubeconfigRequest) (*StoreKubeconfigResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method StoreKubeconfig not implemented")
}
func (UnimplementedKuberServiceServer) DeleteKubeconfig(context.Context, *DeleteKubeconfigRequest) (*DeleteKubeconfigResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteKubeconfig not implemented")
}
func (UnimplementedKuberServiceServer) DeleteNodes(context.Context, *DeleteNodesRequest) (*DeleteNodesResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteNodes not implemented")
}
func (UnimplementedKuberServiceServer) PatchNodes(context.Context, *PatchNodesRequest) (*PatchNodesResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method PatchNodes not implemented")
}
func (UnimplementedKuberServiceServer) SetUpClusterAutoscaler(context.Context, *SetUpClusterAutoscalerRequest) (*SetUpClusterAutoscalerResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetUpClusterAutoscaler not implemented")
}
func (UnimplementedKuberServiceServer) DestroyClusterAutoscaler(context.Context, *DestroyClusterAutoscalerRequest) (*DestroyClusterAutoscalerResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DestroyClusterAutoscaler not implemented")
}
func (UnimplementedKuberServiceServer) PatchClusterInfoConfigMap(context.Context, *PatchClusterInfoConfigMapRequest) (*PatchClusterInfoConfigMapResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method PatchClusterInfoConfigMap not implemented")
}
func (UnimplementedKuberServiceServer) PatchKubeProxyConfigMap(context.Context, *PatchKubeProxyConfigMapRequest) (*PatchKubeProxyConfigMapResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method PatchKubeProxyConfigMap not implemented")
}
func (UnimplementedKuberServiceServer) CiliumRolloutRestart(context.Context, *CiliumRolloutRestartRequest) (*CiliumRolloutRestartResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CiliumRolloutRestart not implemented")
}
func (UnimplementedKuberServiceServer) mustEmbedUnimplementedKuberServiceServer() {}

// UnsafeKuberServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to KuberServiceServer will
// result in compilation errors.
type UnsafeKuberServiceServer interface {
	mustEmbedUnimplementedKuberServiceServer()
}

func RegisterKuberServiceServer(s grpc.ServiceRegistrar, srv KuberServiceServer) {
	s.RegisterService(&KuberService_ServiceDesc, srv)
}

func _KuberService_RemoveLBScrapeConfig_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RemoveLBScrapeConfigRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(KuberServiceServer).RemoveLBScrapeConfig(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/claudie.KuberService/RemoveLBScrapeConfig",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(KuberServiceServer).RemoveLBScrapeConfig(ctx, req.(*RemoveLBScrapeConfigRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _KuberService_StoreLBScrapeConfig_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(StoreLBScrapeConfigRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(KuberServiceServer).StoreLBScrapeConfig(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/claudie.KuberService/StoreLBScrapeConfig",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(KuberServiceServer).StoreLBScrapeConfig(ctx, req.(*StoreLBScrapeConfigRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _KuberService_StoreClusterMetadata_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(StoreClusterMetadataRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(KuberServiceServer).StoreClusterMetadata(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/claudie.KuberService/StoreClusterMetadata",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(KuberServiceServer).StoreClusterMetadata(ctx, req.(*StoreClusterMetadataRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _KuberService_DeleteClusterMetadata_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteClusterMetadataRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(KuberServiceServer).DeleteClusterMetadata(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/claudie.KuberService/DeleteClusterMetadata",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(KuberServiceServer).DeleteClusterMetadata(ctx, req.(*DeleteClusterMetadataRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _KuberService_SetUpStorage_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SetUpStorageRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(KuberServiceServer).SetUpStorage(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/claudie.KuberService/SetUpStorage",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(KuberServiceServer).SetUpStorage(ctx, req.(*SetUpStorageRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _KuberService_StoreKubeconfig_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(StoreKubeconfigRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(KuberServiceServer).StoreKubeconfig(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/claudie.KuberService/StoreKubeconfig",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(KuberServiceServer).StoreKubeconfig(ctx, req.(*StoreKubeconfigRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _KuberService_DeleteKubeconfig_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteKubeconfigRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(KuberServiceServer).DeleteKubeconfig(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/claudie.KuberService/DeleteKubeconfig",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(KuberServiceServer).DeleteKubeconfig(ctx, req.(*DeleteKubeconfigRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _KuberService_DeleteNodes_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteNodesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(KuberServiceServer).DeleteNodes(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/claudie.KuberService/DeleteNodes",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(KuberServiceServer).DeleteNodes(ctx, req.(*DeleteNodesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _KuberService_PatchNodes_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PatchNodesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(KuberServiceServer).PatchNodes(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/claudie.KuberService/PatchNodes",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(KuberServiceServer).PatchNodes(ctx, req.(*PatchNodesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _KuberService_SetUpClusterAutoscaler_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SetUpClusterAutoscalerRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(KuberServiceServer).SetUpClusterAutoscaler(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/claudie.KuberService/SetUpClusterAutoscaler",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(KuberServiceServer).SetUpClusterAutoscaler(ctx, req.(*SetUpClusterAutoscalerRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _KuberService_DestroyClusterAutoscaler_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DestroyClusterAutoscalerRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(KuberServiceServer).DestroyClusterAutoscaler(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/claudie.KuberService/DestroyClusterAutoscaler",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(KuberServiceServer).DestroyClusterAutoscaler(ctx, req.(*DestroyClusterAutoscalerRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _KuberService_PatchClusterInfoConfigMap_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PatchClusterInfoConfigMapRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(KuberServiceServer).PatchClusterInfoConfigMap(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/claudie.KuberService/PatchClusterInfoConfigMap",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(KuberServiceServer).PatchClusterInfoConfigMap(ctx, req.(*PatchClusterInfoConfigMapRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _KuberService_PatchKubeProxyConfigMap_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PatchKubeProxyConfigMapRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(KuberServiceServer).PatchKubeProxyConfigMap(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/claudie.KuberService/PatchKubeProxyConfigMap",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(KuberServiceServer).PatchKubeProxyConfigMap(ctx, req.(*PatchKubeProxyConfigMapRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _KuberService_CiliumRolloutRestart_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CiliumRolloutRestartRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(KuberServiceServer).CiliumRolloutRestart(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/claudie.KuberService/CiliumRolloutRestart",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(KuberServiceServer).CiliumRolloutRestart(ctx, req.(*CiliumRolloutRestartRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// KuberService_ServiceDesc is the grpc.ServiceDesc for KuberService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var KuberService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "claudie.KuberService",
	HandlerType: (*KuberServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "RemoveLBScrapeConfig",
			Handler:    _KuberService_RemoveLBScrapeConfig_Handler,
		},
		{
			MethodName: "StoreLBScrapeConfig",
			Handler:    _KuberService_StoreLBScrapeConfig_Handler,
		},
		{
			MethodName: "StoreClusterMetadata",
			Handler:    _KuberService_StoreClusterMetadata_Handler,
		},
		{
			MethodName: "DeleteClusterMetadata",
			Handler:    _KuberService_DeleteClusterMetadata_Handler,
		},
		{
			MethodName: "SetUpStorage",
			Handler:    _KuberService_SetUpStorage_Handler,
		},
		{
			MethodName: "StoreKubeconfig",
			Handler:    _KuberService_StoreKubeconfig_Handler,
		},
		{
			MethodName: "DeleteKubeconfig",
			Handler:    _KuberService_DeleteKubeconfig_Handler,
		},
		{
			MethodName: "DeleteNodes",
			Handler:    _KuberService_DeleteNodes_Handler,
		},
		{
			MethodName: "PatchNodes",
			Handler:    _KuberService_PatchNodes_Handler,
		},
		{
			MethodName: "SetUpClusterAutoscaler",
			Handler:    _KuberService_SetUpClusterAutoscaler_Handler,
		},
		{
			MethodName: "DestroyClusterAutoscaler",
			Handler:    _KuberService_DestroyClusterAutoscaler_Handler,
		},
		{
			MethodName: "PatchClusterInfoConfigMap",
			Handler:    _KuberService_PatchClusterInfoConfigMap_Handler,
		},
		{
			MethodName: "PatchKubeProxyConfigMap",
			Handler:    _KuberService_PatchKubeProxyConfigMap_Handler,
		},
		{
			MethodName: "CiliumRolloutRestart",
			Handler:    _KuberService_CiliumRolloutRestart_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "kuber.proto",
}
