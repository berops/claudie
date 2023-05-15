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

type SecretWatcher struct {
	usecases *usecases.Usecases
	label    string

	kubeClient *kubernetes.Clientset
	debug      int
}

func NewSecretWatcher(usecases *usecases.Usecases) (*SecretWatcher, error) {
	label, isLabelFound := os.LookupEnv("LABEL")
	if !isLabelFound {
		return nil, fmt.Errorf("environment variable LABEL not found")
	}
	log.Debug().Msgf("Using LABEL %s", label)

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
	}

	return secretWatcher, nil
}

// Monitor will continuously watch for any changes regarding input manifests. This function will exit once usecases context will get canceled.
func (sw *SecretWatcher) Monitor() error {
	// Continuously watch for secrets in current namespace with specified label, until usecases context will be cancelled.
	w, err := sw.kubeClient.CoreV1().Secrets("").Watch(sw.usecases.Context, metav1.ListOptions{LabelSelector: sw.label})
	if err != nil {
		return fmt.Errorf("error while creating WATCH interface : %w", err)
	}

	for event := range w.ResultChan() {
		if secret, ok := event.Object.(*v1.Secret); ok {
			sw.debug++
			log.Info().Msgf("DEBUG: event number %d", sw.debug)
			//TODO REMOVE LATER ^^^
			manifestsData, err := sw.getManifests(secret)
			if err != nil {
				log.Err(err).Msgf("Got error while decoding manifests from secret %s", secret.Name)
			}
			switch event.Type {
			// Added secret
			case watch.Added:
				log.Debug().Msgf("ADDED")
				// All manifest in the secret were added.
				for _, manifest := range manifestsData {
					sw.usecases.SaveChannel <- manifest
				}
			// Modified secret
			case watch.Modified:
				log.Debug().Msgf("MODIFIED")
				configs, err := sw.usecases.ContextBox.GetAllConfigs()
				if err != nil {
					log.Err(err).Msgf("Failed to retrieve configs from Context-box, to verify secret %s modification, skipping...", secret.Name)
					break
				}
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
							sw.usecases.SaveChannel <- sw.getManifest(file, secret.Name, fileName)
						} else {
							// File does not exists, trigger deletion
							log.Debug().Msgf("Assuming file %s from secret %s was removed", fileName, secret.Name)
							sw.usecases.DeleteChannel <- sw.getManifest([]byte(config.Manifest), secret.Name, fileName)
						}
					}
				}
				// Check if any files from secret needs to be saved in DB
				for name, file := range secret.Data {
					// Manifest not in the database yet, save
					if _, ok := inDB[name]; !ok {
						sw.usecases.SaveChannel <- sw.getManifest(file, secret.Name, name)
					}
				}
			// Deleted secret
			case watch.Deleted:
				log.Debug().Msgf("DELETED")
				// All manifest in the secret were deleted.
				for _, manifest := range manifestsData {
					sw.usecases.DeleteChannel <- manifest
				}
			}
		}
	}
	return nil
}

func (sw *SecretWatcher) getManifests(secret *v1.Secret) ([]*usecases.RawManifest, error) {
	manifests := make([]*usecases.RawManifest, 0, len(secret.Data))
	for name, file := range secret.Data {
		content, err := sw.decodeContent(file)
		//TODO make best effort
		if err != nil {
			return nil, err
		}
		manifests = append(manifests, &usecases.RawManifest{
			Manifest:   content,
			SecretName: secret.Name,
			FileName:   name,
		})
	}
	return manifests, nil
}

func (sw *SecretWatcher) getManifest(content []byte, secretName, fileName string) *usecases.RawManifest {
	return &usecases.RawManifest{
		Manifest:   content,
		SecretName: secretName,
		FileName:   fileName,
	}
}

func (sw *SecretWatcher) decodeContent(content []byte) ([]byte, error) {
	decoded := make([]byte, len(content)*(4/3))
	log.Info().Msgf("File: %s", string(content))
	if _, err := base64.StdEncoding.Decode(decoded, content); err != nil {
		// cant use errors.Is as base64 package builds error dynamically in base64.CorruptInputError()
		if strings.Contains(err.Error(), "illegal base64 data") {
			log.Debug().Msgf("File not base64 compatible, assuming it is string data")
			return content, nil
		}
		return nil, err
	}
	return decoded, nil
}
