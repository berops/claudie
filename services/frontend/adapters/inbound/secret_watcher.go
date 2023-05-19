package inboundAdapters

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/berops/claudie/services/frontend/domain/usecases"
	"github.com/rs/zerolog/log"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	watchCreateTimeout = 5 * time.Second
)

// SecretWatcher uses kubernetes API (Watch method) to scan through secrets in the specified namespace.
// If the secret contains input manifest label, it is sent through one of the channels in usecases for processing.
type SecretWatcher struct {
	// usecases is used to pass changed input manifests.
	usecases *usecases.Usecases
	// label is used to identify input manifest secret.
	label string
	// namespace is used to identify namespace where secrets are created.
	namespace string
	// kubeClient is used to communicate with API server.
	kubeClient *kubernetes.Clientset
	// lastResourceVersion is used to resume WATCH API call, after channel is closed.
	lastResourceVersion string
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
	log.Debug().Msgf("Using NAMESPACE %s", ns)

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
func (sw *SecretWatcher) Monitor() {
	// Loop the watching as watch.Interface.ResultChan() gets closed after some time.
	for {
		select {
		case <-sw.usecases.Context.Done():
			// The context is canceled, exit
			return
		default:
			// Create new watcher and monitor.
			w, err := sw.getNewWatcher()
			if err != nil {
				log.Err(err).Msgf("Error while creating WATCH interface, retrying again in %s...", watchCreateTimeout.String())
				time.Sleep(watchCreateTimeout)
				continue
			}
			// Save lastResourceVersion only if its not empty string
			if rsv := sw.monitor(w); rsv != "" {
				sw.lastResourceVersion = rsv
			}
		}
	}
}

// monitor continuously watches for secrets in current namespace with specified label, until usecases context will be cancelled or watch result channel gets closed.
func (sw *SecretWatcher) monitor(w watch.Interface) string {
	rsv := ""
	for event := range w.ResultChan() {
		secret, ok := event.Object.(*v1.Secret)
		if !ok {
			// Skip if event is not Secret type.
			continue
		}
		// Save resource version to continue from there once channel gets closed
		rsv = secret.ResourceVersion
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
				filePathPrefix := fmt.Sprintf("secret_%s.file_", secret.Name)
				if strings.Contains(config.ManifestFileName, filePathPrefix) {
					fileName := strings.TrimPrefix(config.ManifestFileName, filePathPrefix)
					inDB[fileName] = struct{}{}
					//Check if file is present in the modified secret
					if file, ok := secret.Data[fileName]; ok {
						// File exists, save it to Context-box
						log.Debug().Msgf("Assuming file %s from secret %s was modified", fileName, secret.Name)
						manifest, err := sw.getRawManifest(file, secret.Name, fileName)
						if err != nil {
							log.Err(err).Msgf("Failed to decode file %s from secret %s, skipping...", fileName, secret.Name)
							continue
						}
						sw.usecases.SaveChannel <- manifest
					} else {
						// File does not exists, trigger deletion
						log.Debug().Msgf("Assuming file %s from secret %s was removed", fileName, secret.Name)
						manifest, err := sw.getRawManifest(file, secret.Name, fileName)
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
					manifest, err := sw.getRawManifest(file, secret.Name, name)
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
				manifest, err := sw.getRawManifest(file, secret.Name, name)
				if err != nil {
					log.Err(err).Msgf("Failed to decode file %s from secret %s, skipping...", name, secret.Name)
					continue
				}
				sw.usecases.DeleteChannel <- manifest
			}
		case watch.Bookmark, watch.Error:
			// Due to golangci-lint, this case has to be included.
			log.Debug().Msgf("Got event Bookmark or Error; not supported")
		}
	}
	return rsv
}

// getRawManifest returns usecases.RawManifest from the given file.
func (sw *SecretWatcher) getRawManifest(file []byte, secretName, fileName string) (*usecases.RawManifest, error) {
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
		// Cant use errors.Is() as base64 package builds error dynamically via base64.CorruptInputError type
		//nolint:errorlint
		if _, ok := err.(base64.CorruptInputError); ok {
			log.Debug().Msgf("File not base64 compatible, assuming it is string data")
			return content, nil
		}
		return nil, err
	}
	return decoded, nil
}

// getNewWatcher returns newly initialised watch.Interface.
func (sw *SecretWatcher) getNewWatcher() (watch.Interface, error) {
	w, err := sw.kubeClient.CoreV1().Secrets(sw.namespace).Watch(sw.usecases.Context, metav1.ListOptions{LabelSelector: sw.label, Watch: true, ResourceVersion: sw.lastResourceVersion})
	if err != nil {
		return nil, err
	}
	return w, nil
}

// PerformHealthCheck perform health check for secret watcher.
func (sw *SecretWatcher) PerformHealthCheck() error {
	if _, err := sw.getNewWatcher(); err != nil {
		return fmt.Errorf("failed to create WATCH interface : %w", err)
	}
	return nil
}
