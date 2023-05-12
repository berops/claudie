package inboundAdapters

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"

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
	ctx        context.Context
	cancel     context.CancelFunc
}

func NewSecretWatcher(usecases *usecases.Usecases) (*SecretWatcher, error) {
	label, isEnvFound := os.LookupEnv("LABEL")
	if !isEnvFound {
		return nil, fmt.Errorf("environment variable LABEL not found")
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("error while getting in cluster config : %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error while creating client set : %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	secretWatcher := &SecretWatcher{
		usecases:   usecases,
		label:      label,
		kubeClient: clientset,
		ctx:        ctx,
		cancel:     cancel,
	}

	return secretWatcher, nil
}

func (sw *SecretWatcher) Monitor() error {
	// Continuously watch for secrets in current namespace with specified label
	w, err := sw.kubeClient.CoreV1().Secrets("").Watch(sw.ctx, metav1.ListOptions{LabelSelector: sw.label})
	if err != nil {
		return fmt.Errorf("error while creating WATCH interface : %w", err)
	}

	for event := range w.ResultChan() {
		if secret, ok := event.Object.(*v1.Secret); ok {
			manifests, err := sw.getManifests(secret)
			if err != nil {
				log.Err(err).Msgf("Got error while decoding manifests from secret %s", secret.Name)
			}
			switch event.Type {
			case watch.Added:
				// All manifest in the secret were added.
				for _, manifest := range manifests {
					sw.usecases.CreateChannel <- manifest
				}
			case watch.Modified:
				//TODO: find diff between previous and current version

			case watch.Deleted:
				// All manifest in the secret were deleted.
				for _, manifest := range manifests {
					sw.usecases.DeleteChannel <- manifest
				}
			}
		}
	}
	return nil
}

func (sw *SecretWatcher) Stop() error {
	sw.cancel()
	return nil
}

func (sw *SecretWatcher) getManifests(secret *v1.Secret) ([]usecases.RawManifest, error) {
	manifests := make([]usecases.RawManifest, len(secret.Data))
	for name, file := range secret.Data {
		content, err := sw.decodeContent(file)
		//TODO make best effort
		if err != nil {
			return nil, err
		}
		manifests = append(manifests, usecases.RawManifest{
			Manifest:   content,
			SecretName: secret.Name,
			FileName:   name,
		})
	}
	return manifests, nil
}

func (sw *SecretWatcher) decodeContent(content []byte) ([]byte, error) {
	decoded := make([]byte, len(content)*(4/3))
	if _, err := base64.StdEncoding.Decode(decoded, content); err != nil {
		return nil, err
	}
	return decoded, nil
}
