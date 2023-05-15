package inboundAdapters

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/berops/claudie/services/frontend/domain/usecases"
	"github.com/rs/zerolog/log"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// SecretWatcher uses kubernetes API (Watch method) to scan through secrets in the specified namespace.
// If the secret contains input manifest label, it is send through one of the channels in usecases for processing.
type SecretWatcher struct {
	// usecases is used to pass changed input manifests.
	usecases *usecases.Usecases
	// label is used to identify input manifest secret.
	label string
	// namespace is used to identify namespace where secrets are created.
	namespace string
	// kubeClient is used to communicate with API server.
	kubeClient *kubernetes.Clientset
}

// NewSecretWatcher returns new SecretWatcher with initialised variables. Returns error if initialisation failed.
func NewSecretWatcher(usecases *usecases.Usecases) (*SecretWatcher, error) {
	label, isLabelFound := os.LookupEnv("LABEL")
	if !isLabelFound {
		return nil, fmt.Errorf("environment variable LABEL not found")
	}
	log.Debug().Msgf("Using LABEL %s", label)

	ns, isNsFound := os.LookupEnv("NAMESPACE")
	if !isNsFound {
		return nil, fmt.Errorf("environment variable NAMESPACE not found")
	}
	log.Debug().Msgf("Using NAMESPACE %s", label)

	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("error while getting in cluster config : %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error while creating client set : %w", err)
	}
	secretWatcher := &SecretWatcher{
		usecases:   usecases,
		label:      label,
		kubeClient: clientset,
		namespace:  ns,
	}

	return secretWatcher, nil
}

// Monitor will continuously watch for any changes regarding input manifest secrets. This function will exit once usecases context will get canceled.
func (sw *SecretWatcher) Monitor() error {
	// Continuously watch for secrets in current namespace with specified label, until usecases context will be cancelled.
	w, err := sw.kubeClient.CoreV1().Secrets(sw.namespace).Watch(sw.usecases.Context, metav1.ListOptions{LabelSelector: sw.label})
	if err != nil {
		return fmt.Errorf("error while creating WATCH interface : %w", err)
	}

	for event := range w.ResultChan() {
		if secret, ok := event.Object.(*v1.Secret); ok {
			switch event.Type {
			// Modified/Added secret
			case watch.Modified, watch.Added:
				configs, err := sw.usecases.ContextBox.GetAllConfigs()
				if err != nil {
					log.Err(err).Msgf("Failed to retrieve configs from Context-box, to verify secret %s modification, skipping...", secret.Name)
					break
				}
				// Save configs which are already in DB
				inDB := make(map[string]struct{})
				// Check with configs in DB
				for _, config := range configs {
					// Find config which was defined from this secret.
					if strings.Contains(config.ManifestFileName, fmt.Sprintf("secret_%s", secret.Name)) {
						fileName := strings.TrimPrefix(config.ManifestFileName, fmt.Sprintf("secret_%s.file_", secret.Name))
						inDB[fileName] = struct{}{}
						//Check if file is present in the modified secret
						if file, ok := secret.Data[fileName]; ok {
							// File exists, save it to Context-box
							log.Debug().Msgf("Assuming file %s from secret %s was modified", fileName, secret.Name)
							manifest, err := sw.getManifest(file, secret.Name, fileName)
							if err != nil {
								log.Err(err).Msgf("Failed to decode file %s from secret %s, skipping...", fileName, secret.Name)
								continue
							}
							sw.usecases.SaveChannel <- manifest
						} else {
							// File does not exists, trigger deletion
							log.Debug().Msgf("Assuming file %s from secret %s was removed", fileName, secret.Name)
							manifest, err := sw.getManifest(file, secret.Name, fileName)
							if err != nil {
								log.Err(err).Msgf("Failed to decode file %s from secret %s, skipping...", fileName, secret.Name)
								continue
							}
							sw.usecases.DeleteChannel <- manifest
						}
					}
				}
				// Check if any files from secret needs to be saved in DB
				for name, file := range secret.Data {
					// Manifest not in the database yet, save
					if _, ok := inDB[name]; !ok {
						manifest, err := sw.getManifest(file, secret.Name, name)
						if err != nil {
							log.Err(err).Msgf("Failed to decode file %s from secret %s, skipping...", name, secret.Name)
							continue
						}
						sw.usecases.SaveChannel <- manifest
					}
				}
			// Deleted secret
			case watch.Deleted:
				// All manifest in the secret were deleted.
				for name, file := range secret.Data {
					manifest, err := sw.getManifest(file, secret.Name, name)
					if err != nil {
						log.Err(err).Msgf("Failed to decode file %s from secret %s, skipping...", name, secret.Name)
						continue
					}
					sw.usecases.DeleteChannel <- manifest
				}

			}
		}
	}
	return nil
}

// getManifest returns usecases.RawManifest from the given file.
func (sw *SecretWatcher) getManifest(file []byte, secretName, fileName string) (*usecases.RawManifest, error) {
	decoded, err := sw.decodeContent(file)
	if err != nil {
		return nil, err
	}
	return &usecases.RawManifest{
		Manifest:   decoded,
		SecretName: secretName,
		FileName:   fileName,
	}, nil
}

// decodeContent tries to decode base64 files and returns decoded version. If decoding fails,
// it might assume the content is not base64 based and returns original content.
func (sw *SecretWatcher) decodeContent(content []byte) ([]byte, error) {
	decoded := make([]byte, len(content)*(4/3))
	if _, err := base64.StdEncoding.Decode(decoded, content); err != nil {
		// Cant use errors.Is() as base64 package builds error dynamically in base64.CorruptInputError()
		if strings.Contains(err.Error(), "illegal base64 data") {
			log.Debug().Msgf("File not base64 compatible, assuming it is string data")
			return content, nil
		}
		return nil, err
	}
	return decoded, nil
}
