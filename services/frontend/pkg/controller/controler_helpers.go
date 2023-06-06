/*
Copyright 2023 berops.com.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/berops/claudie/internal/manifest"
	v1beta "github.com/berops/claudie/services/frontend/pkg/api/v1beta1"
)

const (
	REQUEUE_NEW    = 30 * time.Second
	REQUEUE_UPDATE = 30 * time.Second
	REQUEUE_DELETE = 30 * time.Second
	REQUEUE_AFTER_ERROR = 30 * time.Second
	REQUEUE_FINISH = 30 * time.Second
)

// crdToManifest takes the v1beta.InputManifest and providersWithSecret and returns a claudie type raw manifest.Manifest type.
// It will combine the manifest.Manifest name as object "Namespace/Name".
func mergeInputManifestWithSecrets(crd v1beta.InputManifest, providersWithSecret []v1beta.ProviderWithData) (manifest.Manifest, error) {
	var providers manifest.Provider

	for _, p := range providersWithSecret {
		switch p.ProviderType {
		case v1beta.GCP:
			gcpCredentials, err := p.GetSecretField(v1beta.GCP_CREDENTIALS)
			if err != nil {
				return manifest.Manifest{}, err
			}
			gcpProject, err := p.GetSecretField(v1beta.GCP_GCP_PROJECT)
			if err != nil {
				return manifest.Manifest{}, err
			}

			providers.GCP = append(providers.GCP, manifest.GCP{
				Name:        p.ProviderName,
				Credentials: gcpCredentials,
				GCPProject:  gcpProject,
			})

		case v1beta.AWS:
			awsAccesskey, err := p.GetSecretField(v1beta.AWS_ACCESS_KEY)
			if err != nil {
				return manifest.Manifest{}, err
			}
			awsSecretkey, err := p.GetSecretField(v1beta.AWS_SECRET_KEY)
			if err != nil {
				return manifest.Manifest{}, err
			}

			providers.AWS = append(providers.AWS, manifest.AWS{
				Name:      p.ProviderName,
				AccessKey: awsAccesskey,
				SecretKey: awsSecretkey,
			})

		case v1beta.HETZNER:
			hetzner_key, err := p.GetSecretField(v1beta.HETZNER_API_TOKEN)
			if err != nil {
				return manifest.Manifest{}, err
			}
			var hetzner = manifest.Hetzner{
				Name:        p.ProviderName,
				Credentials: hetzner_key,
			}
			providers.Hetzner = append(providers.Hetzner, hetzner)
		case v1beta.OCI:
			ociTenant, err := p.GetSecretField(v1beta.OCI_TENANCT_OCID)
			if err != nil {
				return manifest.Manifest{}, err
			}
			ociCompartmentOcid, err := p.GetSecretField(v1beta.OCI_COMPARTMENT_OCID)
			if err != nil {
				return manifest.Manifest{}, err
			}
			ociFingerPrint, err := p.GetSecretField(v1beta.OCI_KEY_FINGERPRINT)
			if err != nil {
				return manifest.Manifest{}, err
			}
			ociPrivateKey, err := p.GetSecretField(v1beta.OCI_PRIVATE_KEY)
			if err != nil {
				return manifest.Manifest{}, err
			}
			ociUserOcid, err := p.GetSecretField(v1beta.OCI_USER_OCID)
			if err != nil {
				return manifest.Manifest{}, err
			}

			providers.OCI = append(providers.OCI, manifest.OCI{
				Name:           p.ProviderName,
				PrivateKey:     ociPrivateKey,
				KeyFingerprint: ociFingerPrint,
				TenancyOCID:    ociTenant,
				CompartmentID:  ociCompartmentOcid,
				UserOCID:       ociUserOcid,
			})
		case v1beta.AZURE:
			azureClientId, err := p.GetSecretField(v1beta.AZURE_CLIENT_ID)
			if err != nil {
				return manifest.Manifest{}, err
			}

			azureClientSecret, err := p.GetSecretField(v1beta.AZURE_CLIENT_SECRET)
			if err != nil {
				return manifest.Manifest{}, err
			}

			azureSubscriptionId, err := p.GetSecretField(v1beta.AZURE_SUBSCRIPTION_ID)
			if err != nil {
				return manifest.Manifest{}, err
			}

			azureTenantId, err := p.GetSecretField(v1beta.AZURE_TENANT_ID)
			if err != nil {
				return manifest.Manifest{}, err
			}

			providers.Azure = append(providers.Azure, manifest.Azure{
				Name:           p.ProviderName,
				SubscriptionId: azureSubscriptionId,
				TenantId:       azureTenantId,
				ClientId:       azureClientId,
				ClientSecret:   azureClientSecret,
			})
		case v1beta.CLOUDFLARE:
			cfApiToken, err := p.GetSecretField(v1beta.CF_API_TOKEN)
			if err != nil {
				return manifest.Manifest{}, err
			}
			providers.Cloudflare = append(providers.Cloudflare, manifest.Cloudflare{
				Name:     p.ProviderName,
				ApiToken: cfApiToken,
			})

		case v1beta.HETZNER_DNS:
			hetznerDNSCredentials, err := p.GetSecretField(v1beta.HETZNER_DNS_CREDENTIALS)
			if err != nil {
				return manifest.Manifest{}, err
			}
			providers.HetznerDNS = append(providers.HetznerDNS, manifest.HetznerDNS{
				Name:     p.ProviderName,
				ApiToken: hetznerDNSCredentials,
			})
		}
	}
	var manifest = manifest.Manifest{
		Name:         crd.GetNamespacedName(),
		Providers:    providers,
		NodePools:    crd.Spec.NodePools,
		Kubernetes:   crd.Spec.Kubernetes,
		LoadBalancer: crd.Spec.LoadBalancer,
	}
	return manifest, nil
}

// getEnvErr take a string representing environment variable as an argument, and returns its value
// If the environment variable is not defined, it will return an error
func getEnvErr(env string) (string, error) {
	value, exists := os.LookupEnv(env)
	if !exists {
		return "", fmt.Errorf("environment variable %s not found", env)
	}
	log.Debug().Msgf("Using %s %s", env, value)

	return value, nil
}
