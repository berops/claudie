// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v5.27.1
// source: spec/provider.proto

package spec

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type GCPProvider struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Key     string `protobuf:"bytes,1,opt,name=key,proto3" json:"key,omitempty"`
	Project string `protobuf:"bytes,2,opt,name=project,proto3" json:"project,omitempty"`
}

func (x *GCPProvider) Reset() {
	*x = GCPProvider{}
	if protoimpl.UnsafeEnabled {
		mi := &file_spec_provider_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GCPProvider) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GCPProvider) ProtoMessage() {}

func (x *GCPProvider) ProtoReflect() protoreflect.Message {
	mi := &file_spec_provider_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GCPProvider.ProtoReflect.Descriptor instead.
func (*GCPProvider) Descriptor() ([]byte, []int) {
	return file_spec_provider_proto_rawDescGZIP(), []int{0}
}

func (x *GCPProvider) GetKey() string {
	if x != nil {
		return x.Key
	}
	return ""
}

func (x *GCPProvider) GetProject() string {
	if x != nil {
		return x.Project
	}
	return ""
}

type HetznerProvider struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Token string `protobuf:"bytes,1,opt,name=token,proto3" json:"token,omitempty"`
}

func (x *HetznerProvider) Reset() {
	*x = HetznerProvider{}
	if protoimpl.UnsafeEnabled {
		mi := &file_spec_provider_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *HetznerProvider) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*HetznerProvider) ProtoMessage() {}

func (x *HetznerProvider) ProtoReflect() protoreflect.Message {
	mi := &file_spec_provider_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use HetznerProvider.ProtoReflect.Descriptor instead.
func (*HetznerProvider) Descriptor() ([]byte, []int) {
	return file_spec_provider_proto_rawDescGZIP(), []int{1}
}

func (x *HetznerProvider) GetToken() string {
	if x != nil {
		return x.Token
	}
	return ""
}

type HetznerDNSProvider struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Token string `protobuf:"bytes,1,opt,name=token,proto3" json:"token,omitempty"`
}

func (x *HetznerDNSProvider) Reset() {
	*x = HetznerDNSProvider{}
	if protoimpl.UnsafeEnabled {
		mi := &file_spec_provider_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *HetznerDNSProvider) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*HetznerDNSProvider) ProtoMessage() {}

func (x *HetznerDNSProvider) ProtoReflect() protoreflect.Message {
	mi := &file_spec_provider_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use HetznerDNSProvider.ProtoReflect.Descriptor instead.
func (*HetznerDNSProvider) Descriptor() ([]byte, []int) {
	return file_spec_provider_proto_rawDescGZIP(), []int{2}
}

func (x *HetznerDNSProvider) GetToken() string {
	if x != nil {
		return x.Token
	}
	return ""
}

type OCIProvider struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	UserOCID        string `protobuf:"bytes,1,opt,name=userOCID,proto3" json:"userOCID,omitempty"`
	TenancyOCID     string `protobuf:"bytes,2,opt,name=tenancyOCID,proto3" json:"tenancyOCID,omitempty"`
	KeyFingerprint  string `protobuf:"bytes,3,opt,name=keyFingerprint,proto3" json:"keyFingerprint,omitempty"`
	CompartmentOCID string `protobuf:"bytes,4,opt,name=compartmentOCID,proto3" json:"compartmentOCID,omitempty"`
	PrivateKey      string `protobuf:"bytes,5,opt,name=privateKey,proto3" json:"privateKey,omitempty"`
}

func (x *OCIProvider) Reset() {
	*x = OCIProvider{}
	if protoimpl.UnsafeEnabled {
		mi := &file_spec_provider_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *OCIProvider) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*OCIProvider) ProtoMessage() {}

func (x *OCIProvider) ProtoReflect() protoreflect.Message {
	mi := &file_spec_provider_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use OCIProvider.ProtoReflect.Descriptor instead.
func (*OCIProvider) Descriptor() ([]byte, []int) {
	return file_spec_provider_proto_rawDescGZIP(), []int{3}
}

func (x *OCIProvider) GetUserOCID() string {
	if x != nil {
		return x.UserOCID
	}
	return ""
}

func (x *OCIProvider) GetTenancyOCID() string {
	if x != nil {
		return x.TenancyOCID
	}
	return ""
}

func (x *OCIProvider) GetKeyFingerprint() string {
	if x != nil {
		return x.KeyFingerprint
	}
	return ""
}

func (x *OCIProvider) GetCompartmentOCID() string {
	if x != nil {
		return x.CompartmentOCID
	}
	return ""
}

func (x *OCIProvider) GetPrivateKey() string {
	if x != nil {
		return x.PrivateKey
	}
	return ""
}

type AWSProvider struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	SecretKey string `protobuf:"bytes,1,opt,name=secretKey,proto3" json:"secretKey,omitempty"`
	AccessKey string `protobuf:"bytes,2,opt,name=accessKey,proto3" json:"accessKey,omitempty"`
}

func (x *AWSProvider) Reset() {
	*x = AWSProvider{}
	if protoimpl.UnsafeEnabled {
		mi := &file_spec_provider_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *AWSProvider) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AWSProvider) ProtoMessage() {}

func (x *AWSProvider) ProtoReflect() protoreflect.Message {
	mi := &file_spec_provider_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AWSProvider.ProtoReflect.Descriptor instead.
func (*AWSProvider) Descriptor() ([]byte, []int) {
	return file_spec_provider_proto_rawDescGZIP(), []int{4}
}

func (x *AWSProvider) GetSecretKey() string {
	if x != nil {
		return x.SecretKey
	}
	return ""
}

func (x *AWSProvider) GetAccessKey() string {
	if x != nil {
		return x.AccessKey
	}
	return ""
}

type AzureProvider struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	SubscriptionID string `protobuf:"bytes,1,opt,name=subscriptionID,proto3" json:"subscriptionID,omitempty"`
	TenantID       string `protobuf:"bytes,2,opt,name=tenantID,proto3" json:"tenantID,omitempty"`
	ClientID       string `protobuf:"bytes,3,opt,name=clientID,proto3" json:"clientID,omitempty"`
	ClientSecret   string `protobuf:"bytes,4,opt,name=clientSecret,proto3" json:"clientSecret,omitempty"`
}

func (x *AzureProvider) Reset() {
	*x = AzureProvider{}
	if protoimpl.UnsafeEnabled {
		mi := &file_spec_provider_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *AzureProvider) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AzureProvider) ProtoMessage() {}

func (x *AzureProvider) ProtoReflect() protoreflect.Message {
	mi := &file_spec_provider_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AzureProvider.ProtoReflect.Descriptor instead.
func (*AzureProvider) Descriptor() ([]byte, []int) {
	return file_spec_provider_proto_rawDescGZIP(), []int{5}
}

func (x *AzureProvider) GetSubscriptionID() string {
	if x != nil {
		return x.SubscriptionID
	}
	return ""
}

func (x *AzureProvider) GetTenantID() string {
	if x != nil {
		return x.TenantID
	}
	return ""
}

func (x *AzureProvider) GetClientID() string {
	if x != nil {
		return x.ClientID
	}
	return ""
}

func (x *AzureProvider) GetClientSecret() string {
	if x != nil {
		return x.ClientSecret
	}
	return ""
}

type CloudflareProvider struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Token string `protobuf:"bytes,1,opt,name=token,proto3" json:"token,omitempty"`
}

func (x *CloudflareProvider) Reset() {
	*x = CloudflareProvider{}
	if protoimpl.UnsafeEnabled {
		mi := &file_spec_provider_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CloudflareProvider) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CloudflareProvider) ProtoMessage() {}

func (x *CloudflareProvider) ProtoReflect() protoreflect.Message {
	mi := &file_spec_provider_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CloudflareProvider.ProtoReflect.Descriptor instead.
func (*CloudflareProvider) Descriptor() ([]byte, []int) {
	return file_spec_provider_proto_rawDescGZIP(), []int{6}
}

func (x *CloudflareProvider) GetToken() string {
	if x != nil {
		return x.Token
	}
	return ""
}

type GenesisCloudProvider struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Token string `protobuf:"bytes,1,opt,name=token,proto3" json:"token,omitempty"`
}

func (x *GenesisCloudProvider) Reset() {
	*x = GenesisCloudProvider{}
	if protoimpl.UnsafeEnabled {
		mi := &file_spec_provider_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GenesisCloudProvider) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GenesisCloudProvider) ProtoMessage() {}

func (x *GenesisCloudProvider) ProtoReflect() protoreflect.Message {
	mi := &file_spec_provider_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GenesisCloudProvider.ProtoReflect.Descriptor instead.
func (*GenesisCloudProvider) Descriptor() ([]byte, []int) {
	return file_spec_provider_proto_rawDescGZIP(), []int{7}
}

func (x *GenesisCloudProvider) GetToken() string {
	if x != nil {
		return x.Token
	}
	return ""
}

type Provider struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	SpecName          string `protobuf:"bytes,1,opt,name=specName,proto3" json:"specName,omitempty"`
	CloudProviderName string `protobuf:"bytes,2,opt,name=cloudProviderName,proto3" json:"cloudProviderName,omitempty"`
	// Types that are assignable to ProviderType:
	//
	//	*Provider_Gcp
	//	*Provider_Hetzner
	//	*Provider_Hetznerdns
	//	*Provider_Oci
	//	*Provider_Aws
	//	*Provider_Azure
	//	*Provider_Cloudflare
	//	*Provider_Genesiscloud
	ProviderType isProvider_ProviderType `protobuf_oneof:"ProviderType"`
	Templates    *TemplateRepository     `protobuf:"bytes,13,opt,name=templates,proto3" json:"templates,omitempty"`
}

func (x *Provider) Reset() {
	*x = Provider{}
	if protoimpl.UnsafeEnabled {
		mi := &file_spec_provider_proto_msgTypes[8]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Provider) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Provider) ProtoMessage() {}

func (x *Provider) ProtoReflect() protoreflect.Message {
	mi := &file_spec_provider_proto_msgTypes[8]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Provider.ProtoReflect.Descriptor instead.
func (*Provider) Descriptor() ([]byte, []int) {
	return file_spec_provider_proto_rawDescGZIP(), []int{8}
}

func (x *Provider) GetSpecName() string {
	if x != nil {
		return x.SpecName
	}
	return ""
}

func (x *Provider) GetCloudProviderName() string {
	if x != nil {
		return x.CloudProviderName
	}
	return ""
}

func (m *Provider) GetProviderType() isProvider_ProviderType {
	if m != nil {
		return m.ProviderType
	}
	return nil
}

func (x *Provider) GetGcp() *GCPProvider {
	if x, ok := x.GetProviderType().(*Provider_Gcp); ok {
		return x.Gcp
	}
	return nil
}

func (x *Provider) GetHetzner() *HetznerProvider {
	if x, ok := x.GetProviderType().(*Provider_Hetzner); ok {
		return x.Hetzner
	}
	return nil
}

func (x *Provider) GetHetznerdns() *HetznerDNSProvider {
	if x, ok := x.GetProviderType().(*Provider_Hetznerdns); ok {
		return x.Hetznerdns
	}
	return nil
}

func (x *Provider) GetOci() *OCIProvider {
	if x, ok := x.GetProviderType().(*Provider_Oci); ok {
		return x.Oci
	}
	return nil
}

func (x *Provider) GetAws() *AWSProvider {
	if x, ok := x.GetProviderType().(*Provider_Aws); ok {
		return x.Aws
	}
	return nil
}

func (x *Provider) GetAzure() *AzureProvider {
	if x, ok := x.GetProviderType().(*Provider_Azure); ok {
		return x.Azure
	}
	return nil
}

func (x *Provider) GetCloudflare() *CloudflareProvider {
	if x, ok := x.GetProviderType().(*Provider_Cloudflare); ok {
		return x.Cloudflare
	}
	return nil
}

func (x *Provider) GetGenesiscloud() *GenesisCloudProvider {
	if x, ok := x.GetProviderType().(*Provider_Genesiscloud); ok {
		return x.Genesiscloud
	}
	return nil
}

func (x *Provider) GetTemplates() *TemplateRepository {
	if x != nil {
		return x.Templates
	}
	return nil
}

type isProvider_ProviderType interface {
	isProvider_ProviderType()
}

type Provider_Gcp struct {
	Gcp *GCPProvider `protobuf:"bytes,3,opt,name=gcp,proto3,oneof"`
}

type Provider_Hetzner struct {
	Hetzner *HetznerProvider `protobuf:"bytes,4,opt,name=hetzner,proto3,oneof"`
}

type Provider_Hetznerdns struct {
	Hetznerdns *HetznerDNSProvider `protobuf:"bytes,5,opt,name=hetznerdns,proto3,oneof"`
}

type Provider_Oci struct {
	Oci *OCIProvider `protobuf:"bytes,6,opt,name=oci,proto3,oneof"`
}

type Provider_Aws struct {
	Aws *AWSProvider `protobuf:"bytes,7,opt,name=aws,proto3,oneof"`
}

type Provider_Azure struct {
	Azure *AzureProvider `protobuf:"bytes,8,opt,name=azure,proto3,oneof"`
}

type Provider_Cloudflare struct {
	Cloudflare *CloudflareProvider `protobuf:"bytes,9,opt,name=cloudflare,proto3,oneof"`
}

type Provider_Genesiscloud struct {
	Genesiscloud *GenesisCloudProvider `protobuf:"bytes,10,opt,name=genesiscloud,proto3,oneof"`
}

func (*Provider_Gcp) isProvider_ProviderType() {}

func (*Provider_Hetzner) isProvider_ProviderType() {}

func (*Provider_Hetznerdns) isProvider_ProviderType() {}

func (*Provider_Oci) isProvider_ProviderType() {}

func (*Provider_Aws) isProvider_ProviderType() {}

func (*Provider_Azure) isProvider_ProviderType() {}

func (*Provider_Cloudflare) isProvider_ProviderType() {}

func (*Provider_Genesiscloud) isProvider_ProviderType() {}

type TemplateRepository struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Repository string  `protobuf:"bytes,1,opt,name=repository,proto3" json:"repository,omitempty"`
	Tag        *string `protobuf:"bytes,2,opt,name=tag,proto3,oneof" json:"tag,omitempty"`
	Path       string  `protobuf:"bytes,3,opt,name=path,proto3" json:"path,omitempty"`
	CommitHash string  `protobuf:"bytes,4,opt,name=commitHash,proto3" json:"commitHash,omitempty"`
}

func (x *TemplateRepository) Reset() {
	*x = TemplateRepository{}
	if protoimpl.UnsafeEnabled {
		mi := &file_spec_provider_proto_msgTypes[9]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TemplateRepository) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TemplateRepository) ProtoMessage() {}

func (x *TemplateRepository) ProtoReflect() protoreflect.Message {
	mi := &file_spec_provider_proto_msgTypes[9]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TemplateRepository.ProtoReflect.Descriptor instead.
func (*TemplateRepository) Descriptor() ([]byte, []int) {
	return file_spec_provider_proto_rawDescGZIP(), []int{9}
}

func (x *TemplateRepository) GetRepository() string {
	if x != nil {
		return x.Repository
	}
	return ""
}

func (x *TemplateRepository) GetTag() string {
	if x != nil && x.Tag != nil {
		return *x.Tag
	}
	return ""
}

func (x *TemplateRepository) GetPath() string {
	if x != nil {
		return x.Path
	}
	return ""
}

func (x *TemplateRepository) GetCommitHash() string {
	if x != nil {
		return x.CommitHash
	}
	return ""
}

var File_spec_provider_proto protoreflect.FileDescriptor

var file_spec_provider_proto_rawDesc = []byte{
	0x0a, 0x13, 0x73, 0x70, 0x65, 0x63, 0x2f, 0x70, 0x72, 0x6f, 0x76, 0x69, 0x64, 0x65, 0x72, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x04, 0x73, 0x70, 0x65, 0x63, 0x22, 0x39, 0x0a, 0x0b, 0x47,
	0x43, 0x50, 0x50, 0x72, 0x6f, 0x76, 0x69, 0x64, 0x65, 0x72, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65,
	0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x18, 0x0a, 0x07,
	0x70, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x70,
	0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x22, 0x27, 0x0a, 0x0f, 0x48, 0x65, 0x74, 0x7a, 0x6e, 0x65,
	0x72, 0x50, 0x72, 0x6f, 0x76, 0x69, 0x64, 0x65, 0x72, 0x12, 0x14, 0x0a, 0x05, 0x74, 0x6f, 0x6b,
	0x65, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x74, 0x6f, 0x6b, 0x65, 0x6e, 0x22,
	0x2a, 0x0a, 0x12, 0x48, 0x65, 0x74, 0x7a, 0x6e, 0x65, 0x72, 0x44, 0x4e, 0x53, 0x50, 0x72, 0x6f,
	0x76, 0x69, 0x64, 0x65, 0x72, 0x12, 0x14, 0x0a, 0x05, 0x74, 0x6f, 0x6b, 0x65, 0x6e, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x74, 0x6f, 0x6b, 0x65, 0x6e, 0x22, 0xbd, 0x01, 0x0a, 0x0b,
	0x4f, 0x43, 0x49, 0x50, 0x72, 0x6f, 0x76, 0x69, 0x64, 0x65, 0x72, 0x12, 0x1a, 0x0a, 0x08, 0x75,
	0x73, 0x65, 0x72, 0x4f, 0x43, 0x49, 0x44, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x75,
	0x73, 0x65, 0x72, 0x4f, 0x43, 0x49, 0x44, 0x12, 0x20, 0x0a, 0x0b, 0x74, 0x65, 0x6e, 0x61, 0x6e,
	0x63, 0x79, 0x4f, 0x43, 0x49, 0x44, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x74, 0x65,
	0x6e, 0x61, 0x6e, 0x63, 0x79, 0x4f, 0x43, 0x49, 0x44, 0x12, 0x26, 0x0a, 0x0e, 0x6b, 0x65, 0x79,
	0x46, 0x69, 0x6e, 0x67, 0x65, 0x72, 0x70, 0x72, 0x69, 0x6e, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x0e, 0x6b, 0x65, 0x79, 0x46, 0x69, 0x6e, 0x67, 0x65, 0x72, 0x70, 0x72, 0x69, 0x6e,
	0x74, 0x12, 0x28, 0x0a, 0x0f, 0x63, 0x6f, 0x6d, 0x70, 0x61, 0x72, 0x74, 0x6d, 0x65, 0x6e, 0x74,
	0x4f, 0x43, 0x49, 0x44, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0f, 0x63, 0x6f, 0x6d, 0x70,
	0x61, 0x72, 0x74, 0x6d, 0x65, 0x6e, 0x74, 0x4f, 0x43, 0x49, 0x44, 0x12, 0x1e, 0x0a, 0x0a, 0x70,
	0x72, 0x69, 0x76, 0x61, 0x74, 0x65, 0x4b, 0x65, 0x79, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x0a, 0x70, 0x72, 0x69, 0x76, 0x61, 0x74, 0x65, 0x4b, 0x65, 0x79, 0x22, 0x49, 0x0a, 0x0b, 0x41,
	0x57, 0x53, 0x50, 0x72, 0x6f, 0x76, 0x69, 0x64, 0x65, 0x72, 0x12, 0x1c, 0x0a, 0x09, 0x73, 0x65,
	0x63, 0x72, 0x65, 0x74, 0x4b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x73,
	0x65, 0x63, 0x72, 0x65, 0x74, 0x4b, 0x65, 0x79, 0x12, 0x1c, 0x0a, 0x09, 0x61, 0x63, 0x63, 0x65,
	0x73, 0x73, 0x4b, 0x65, 0x79, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x61, 0x63, 0x63,
	0x65, 0x73, 0x73, 0x4b, 0x65, 0x79, 0x22, 0x93, 0x01, 0x0a, 0x0d, 0x41, 0x7a, 0x75, 0x72, 0x65,
	0x50, 0x72, 0x6f, 0x76, 0x69, 0x64, 0x65, 0x72, 0x12, 0x26, 0x0a, 0x0e, 0x73, 0x75, 0x62, 0x73,
	0x63, 0x72, 0x69, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x49, 0x44, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x0e, 0x73, 0x75, 0x62, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x49, 0x44,
	0x12, 0x1a, 0x0a, 0x08, 0x74, 0x65, 0x6e, 0x61, 0x6e, 0x74, 0x49, 0x44, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x08, 0x74, 0x65, 0x6e, 0x61, 0x6e, 0x74, 0x49, 0x44, 0x12, 0x1a, 0x0a, 0x08,
	0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x49, 0x44, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08,
	0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x49, 0x44, 0x12, 0x22, 0x0a, 0x0c, 0x63, 0x6c, 0x69, 0x65,
	0x6e, 0x74, 0x53, 0x65, 0x63, 0x72, 0x65, 0x74, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0c,
	0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x53, 0x65, 0x63, 0x72, 0x65, 0x74, 0x22, 0x2a, 0x0a, 0x12,
	0x43, 0x6c, 0x6f, 0x75, 0x64, 0x66, 0x6c, 0x61, 0x72, 0x65, 0x50, 0x72, 0x6f, 0x76, 0x69, 0x64,
	0x65, 0x72, 0x12, 0x14, 0x0a, 0x05, 0x74, 0x6f, 0x6b, 0x65, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x05, 0x74, 0x6f, 0x6b, 0x65, 0x6e, 0x22, 0x2c, 0x0a, 0x14, 0x47, 0x65, 0x6e, 0x65,
	0x73, 0x69, 0x73, 0x43, 0x6c, 0x6f, 0x75, 0x64, 0x50, 0x72, 0x6f, 0x76, 0x69, 0x64, 0x65, 0x72,
	0x12, 0x14, 0x0a, 0x05, 0x74, 0x6f, 0x6b, 0x65, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x05, 0x74, 0x6f, 0x6b, 0x65, 0x6e, 0x22, 0xab, 0x04, 0x0a, 0x08, 0x50, 0x72, 0x6f, 0x76, 0x69,
	0x64, 0x65, 0x72, 0x12, 0x1a, 0x0a, 0x08, 0x73, 0x70, 0x65, 0x63, 0x4e, 0x61, 0x6d, 0x65, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x73, 0x70, 0x65, 0x63, 0x4e, 0x61, 0x6d, 0x65, 0x12,
	0x2c, 0x0a, 0x11, 0x63, 0x6c, 0x6f, 0x75, 0x64, 0x50, 0x72, 0x6f, 0x76, 0x69, 0x64, 0x65, 0x72,
	0x4e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x11, 0x63, 0x6c, 0x6f, 0x75,
	0x64, 0x50, 0x72, 0x6f, 0x76, 0x69, 0x64, 0x65, 0x72, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x25, 0x0a,
	0x03, 0x67, 0x63, 0x70, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x11, 0x2e, 0x73, 0x70, 0x65,
	0x63, 0x2e, 0x47, 0x43, 0x50, 0x50, 0x72, 0x6f, 0x76, 0x69, 0x64, 0x65, 0x72, 0x48, 0x00, 0x52,
	0x03, 0x67, 0x63, 0x70, 0x12, 0x31, 0x0a, 0x07, 0x68, 0x65, 0x74, 0x7a, 0x6e, 0x65, 0x72, 0x18,
	0x04, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x15, 0x2e, 0x73, 0x70, 0x65, 0x63, 0x2e, 0x48, 0x65, 0x74,
	0x7a, 0x6e, 0x65, 0x72, 0x50, 0x72, 0x6f, 0x76, 0x69, 0x64, 0x65, 0x72, 0x48, 0x00, 0x52, 0x07,
	0x68, 0x65, 0x74, 0x7a, 0x6e, 0x65, 0x72, 0x12, 0x3a, 0x0a, 0x0a, 0x68, 0x65, 0x74, 0x7a, 0x6e,
	0x65, 0x72, 0x64, 0x6e, 0x73, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x18, 0x2e, 0x73, 0x70,
	0x65, 0x63, 0x2e, 0x48, 0x65, 0x74, 0x7a, 0x6e, 0x65, 0x72, 0x44, 0x4e, 0x53, 0x50, 0x72, 0x6f,
	0x76, 0x69, 0x64, 0x65, 0x72, 0x48, 0x00, 0x52, 0x0a, 0x68, 0x65, 0x74, 0x7a, 0x6e, 0x65, 0x72,
	0x64, 0x6e, 0x73, 0x12, 0x25, 0x0a, 0x03, 0x6f, 0x63, 0x69, 0x18, 0x06, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x11, 0x2e, 0x73, 0x70, 0x65, 0x63, 0x2e, 0x4f, 0x43, 0x49, 0x50, 0x72, 0x6f, 0x76, 0x69,
	0x64, 0x65, 0x72, 0x48, 0x00, 0x52, 0x03, 0x6f, 0x63, 0x69, 0x12, 0x25, 0x0a, 0x03, 0x61, 0x77,
	0x73, 0x18, 0x07, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x11, 0x2e, 0x73, 0x70, 0x65, 0x63, 0x2e, 0x41,
	0x57, 0x53, 0x50, 0x72, 0x6f, 0x76, 0x69, 0x64, 0x65, 0x72, 0x48, 0x00, 0x52, 0x03, 0x61, 0x77,
	0x73, 0x12, 0x2b, 0x0a, 0x05, 0x61, 0x7a, 0x75, 0x72, 0x65, 0x18, 0x08, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x13, 0x2e, 0x73, 0x70, 0x65, 0x63, 0x2e, 0x41, 0x7a, 0x75, 0x72, 0x65, 0x50, 0x72, 0x6f,
	0x76, 0x69, 0x64, 0x65, 0x72, 0x48, 0x00, 0x52, 0x05, 0x61, 0x7a, 0x75, 0x72, 0x65, 0x12, 0x3a,
	0x0a, 0x0a, 0x63, 0x6c, 0x6f, 0x75, 0x64, 0x66, 0x6c, 0x61, 0x72, 0x65, 0x18, 0x09, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x18, 0x2e, 0x73, 0x70, 0x65, 0x63, 0x2e, 0x43, 0x6c, 0x6f, 0x75, 0x64, 0x66,
	0x6c, 0x61, 0x72, 0x65, 0x50, 0x72, 0x6f, 0x76, 0x69, 0x64, 0x65, 0x72, 0x48, 0x00, 0x52, 0x0a,
	0x63, 0x6c, 0x6f, 0x75, 0x64, 0x66, 0x6c, 0x61, 0x72, 0x65, 0x12, 0x40, 0x0a, 0x0c, 0x67, 0x65,
	0x6e, 0x65, 0x73, 0x69, 0x73, 0x63, 0x6c, 0x6f, 0x75, 0x64, 0x18, 0x0a, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x1a, 0x2e, 0x73, 0x70, 0x65, 0x63, 0x2e, 0x47, 0x65, 0x6e, 0x65, 0x73, 0x69, 0x73, 0x43,
	0x6c, 0x6f, 0x75, 0x64, 0x50, 0x72, 0x6f, 0x76, 0x69, 0x64, 0x65, 0x72, 0x48, 0x00, 0x52, 0x0c,
	0x67, 0x65, 0x6e, 0x65, 0x73, 0x69, 0x73, 0x63, 0x6c, 0x6f, 0x75, 0x64, 0x12, 0x36, 0x0a, 0x09,
	0x74, 0x65, 0x6d, 0x70, 0x6c, 0x61, 0x74, 0x65, 0x73, 0x18, 0x0d, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x18, 0x2e, 0x73, 0x70, 0x65, 0x63, 0x2e, 0x54, 0x65, 0x6d, 0x70, 0x6c, 0x61, 0x74, 0x65, 0x52,
	0x65, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x6f, 0x72, 0x79, 0x52, 0x09, 0x74, 0x65, 0x6d, 0x70, 0x6c,
	0x61, 0x74, 0x65, 0x73, 0x42, 0x0e, 0x0a, 0x0c, 0x50, 0x72, 0x6f, 0x76, 0x69, 0x64, 0x65, 0x72,
	0x54, 0x79, 0x70, 0x65, 0x22, 0x87, 0x01, 0x0a, 0x12, 0x54, 0x65, 0x6d, 0x70, 0x6c, 0x61, 0x74,
	0x65, 0x52, 0x65, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x6f, 0x72, 0x79, 0x12, 0x1e, 0x0a, 0x0a, 0x72,
	0x65, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x6f, 0x72, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x0a, 0x72, 0x65, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x6f, 0x72, 0x79, 0x12, 0x15, 0x0a, 0x03, 0x74,
	0x61, 0x67, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x48, 0x00, 0x52, 0x03, 0x74, 0x61, 0x67, 0x88,
	0x01, 0x01, 0x12, 0x12, 0x0a, 0x04, 0x70, 0x61, 0x74, 0x68, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x04, 0x70, 0x61, 0x74, 0x68, 0x12, 0x1e, 0x0a, 0x0a, 0x63, 0x6f, 0x6d, 0x6d, 0x69, 0x74,
	0x48, 0x61, 0x73, 0x68, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x63, 0x6f, 0x6d, 0x6d,
	0x69, 0x74, 0x48, 0x61, 0x73, 0x68, 0x42, 0x06, 0x0a, 0x04, 0x5f, 0x74, 0x61, 0x67, 0x42, 0x29,
	0x5a, 0x27, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x62, 0x65, 0x72,
	0x6f, 0x70, 0x73, 0x2f, 0x63, 0x6c, 0x61, 0x75, 0x64, 0x69, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x2f, 0x70, 0x62, 0x2f, 0x73, 0x70, 0x65, 0x63, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x33,
}

var (
	file_spec_provider_proto_rawDescOnce sync.Once
	file_spec_provider_proto_rawDescData = file_spec_provider_proto_rawDesc
)

func file_spec_provider_proto_rawDescGZIP() []byte {
	file_spec_provider_proto_rawDescOnce.Do(func() {
		file_spec_provider_proto_rawDescData = protoimpl.X.CompressGZIP(file_spec_provider_proto_rawDescData)
	})
	return file_spec_provider_proto_rawDescData
}

var file_spec_provider_proto_msgTypes = make([]protoimpl.MessageInfo, 10)
var file_spec_provider_proto_goTypes = []interface{}{
	(*GCPProvider)(nil),          // 0: spec.GCPProvider
	(*HetznerProvider)(nil),      // 1: spec.HetznerProvider
	(*HetznerDNSProvider)(nil),   // 2: spec.HetznerDNSProvider
	(*OCIProvider)(nil),          // 3: spec.OCIProvider
	(*AWSProvider)(nil),          // 4: spec.AWSProvider
	(*AzureProvider)(nil),        // 5: spec.AzureProvider
	(*CloudflareProvider)(nil),   // 6: spec.CloudflareProvider
	(*GenesisCloudProvider)(nil), // 7: spec.GenesisCloudProvider
	(*Provider)(nil),             // 8: spec.Provider
	(*TemplateRepository)(nil),   // 9: spec.TemplateRepository
}
var file_spec_provider_proto_depIdxs = []int32{
	0, // 0: spec.Provider.gcp:type_name -> spec.GCPProvider
	1, // 1: spec.Provider.hetzner:type_name -> spec.HetznerProvider
	2, // 2: spec.Provider.hetznerdns:type_name -> spec.HetznerDNSProvider
	3, // 3: spec.Provider.oci:type_name -> spec.OCIProvider
	4, // 4: spec.Provider.aws:type_name -> spec.AWSProvider
	5, // 5: spec.Provider.azure:type_name -> spec.AzureProvider
	6, // 6: spec.Provider.cloudflare:type_name -> spec.CloudflareProvider
	7, // 7: spec.Provider.genesiscloud:type_name -> spec.GenesisCloudProvider
	9, // 8: spec.Provider.templates:type_name -> spec.TemplateRepository
	9, // [9:9] is the sub-list for method output_type
	9, // [9:9] is the sub-list for method input_type
	9, // [9:9] is the sub-list for extension type_name
	9, // [9:9] is the sub-list for extension extendee
	0, // [0:9] is the sub-list for field type_name
}

func init() { file_spec_provider_proto_init() }
func file_spec_provider_proto_init() {
	if File_spec_provider_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_spec_provider_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GCPProvider); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_spec_provider_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*HetznerProvider); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_spec_provider_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*HetznerDNSProvider); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_spec_provider_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*OCIProvider); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_spec_provider_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*AWSProvider); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_spec_provider_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*AzureProvider); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_spec_provider_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CloudflareProvider); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_spec_provider_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GenesisCloudProvider); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_spec_provider_proto_msgTypes[8].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Provider); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_spec_provider_proto_msgTypes[9].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*TemplateRepository); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	file_spec_provider_proto_msgTypes[8].OneofWrappers = []interface{}{
		(*Provider_Gcp)(nil),
		(*Provider_Hetzner)(nil),
		(*Provider_Hetznerdns)(nil),
		(*Provider_Oci)(nil),
		(*Provider_Aws)(nil),
		(*Provider_Azure)(nil),
		(*Provider_Cloudflare)(nil),
		(*Provider_Genesiscloud)(nil),
	}
	file_spec_provider_proto_msgTypes[9].OneofWrappers = []interface{}{}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_spec_provider_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   10,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_spec_provider_proto_goTypes,
		DependencyIndexes: file_spec_provider_proto_depIdxs,
		MessageInfos:      file_spec_provider_proto_msgTypes,
	}.Build()
	File_spec_provider_proto = out.File
	file_spec_provider_proto_rawDesc = nil
	file_spec_provider_proto_goTypes = nil
	file_spec_provider_proto_depIdxs = nil
}
