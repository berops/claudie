package nodes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strings"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/nodes"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

const patchProviderIDPathFormat = "{\"spec\":{\"providerID\":\"%s\"}}"

type patchJson struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value,omitempty"`
}

type PatchAnnotations struct {
	MetadataAnnotations `json:"metadata"`
}

type MetadataAnnotations struct {
	Annotations map[string]string `json:"annotations"`
}

type patchData struct {
	kBase        kubectl.Kubectl
	clusterID    string
	wg           *errgroup.Group
	processLimit *semaphore.Weighted
	errChan      chan<- error
}

func Patch(
	logger zerolog.Logger,
	patch *spec.Update_KuberPatchNodes,
	cluster *spec.K8Scluster,
	processLimit *semaphore.Weighted,
	workersLimit int,
) error {
	var (
		clusterID = cluster.ClusterInfo.Id()
		errChan   = make(chan error)
		kbase     = kubectl.Kubectl{
			Kubeconfig:        cluster.Kubeconfig,
			MaxKubectlRetries: 3,
		}

		aggregateDone  = make(chan struct{})
		aggregateError error
	)

	go func() {
		defer close(aggregateDone)
		for err := range errChan {
			aggregateError = errors.Join(aggregateError, err)
		}
	}()

	// 1. re-apply existing state with new additions.
	wg, ctx := errgroup.WithContext(context.Background())
	wg.SetLimit(workersLimit)

	p := patchData{
		kBase:        kbase,
		clusterID:    clusterID,
		wg:           wg,
		processLimit: processLimit,
		errChan:      errChan,
	}

	for _, np := range cluster.ClusterInfo.NodePools {
		patchProviderID(ctx, logger, p, np)

		newAnnotations := patch.Add.GetAnnotations()[np.Name].GetAnnotations()
		annotateNodePool(ctx, logger, p, np, newAnnotations)

		newLabels := patch.Add.GetLabels()[np.Name].GetLabels()
		labelNodePool(ctx, logger, np, p, newLabels)

		newTaints := patch.Add.GetTaints()[np.Name].GetTaints()
		taintNodePool(ctx, logger, np, p, newTaints)
	}

	if err := wg.Wait(); err != nil {
		errChan <- err
	}

	// 2. delete
	wg, ctx = errgroup.WithContext(context.Background())
	wg.SetLimit(workersLimit)
	p = patchData{
		kBase:        kbase,
		clusterID:    clusterID,
		wg:           wg,
		processLimit: processLimit,
		errChan:      errChan,
	}
	for _, np := range cluster.ClusterInfo.NodePools {
		if v := patch.Remove.GetLabels()[np.Name]; len(v.GetLabels()) > 0 {
			removeLabels(ctx, logger, np, p, v.Labels)
		}
		if v := patch.Remove.GetAnnotations()[np.Name]; len(v.GetAnnotations()) > 0 {
			removeAnnotations(ctx, logger, np, p, v.Annotations)
		}
		if v := patch.Remove.GetTaints()[np.Name]; len(v.GetTaints()) > 0 {
			removeTaints(ctx, logger, np, p, v.Taints)
		}
	}

	if err := wg.Wait(); err != nil {
		errChan <- err
	}

	close(errChan)
	<-aggregateDone
	return aggregateError
}

func patchProviderID(
	ctx context.Context,
	logger zerolog.Logger,
	p patchData,
	np *spec.NodePool,
) {
	for _, node := range np.Nodes {
		nodeName := strings.TrimPrefix(node.Name, fmt.Sprintf("%s-", p.clusterID))
		patchPath := fmt.Sprintf(patchProviderIDPathFormat, fmt.Sprintf(nodes.ProviderIdFormat, nodeName))

		kc := p.kBase
		kc.Stdout = comm.GetStdOut(p.clusterID)
		kc.Stderr = comm.GetStdErr(p.clusterID)

		p.wg.Go(func() error {
			if err := p.processLimit.Acquire(ctx, 1); err != nil {
				p.errChan <- fmt.Errorf("error while patching node, failed to acquire sempahore: %w", err)
				return nil
			}
			defer p.processLimit.Release(1)

			if err := kc.KubectlPatch("node", nodeName, patchPath); err != nil {
				logger.Err(err).Str("node", nodeName).Msgf("Error while patching node with patch %s", patchPath)
				p.errChan <- fmt.Errorf("error while patching one or more nodes with providerID")
				// fallthrough
			}
			return nil
		})
	}
}

func removeLabels(
	ctx context.Context,
	logger zerolog.Logger,
	np *spec.NodePool,
	p patchData,
	toRemove []string,
) {
	name := np.Name
	for _, key := range toRemove {
		escapedKey := strings.ReplaceAll(key, "/", "~1")
		patchPath, err := buildJSONPatchString("remove", "/metadata/labels/"+escapedKey, nil)
		if err != nil {
			p.errChan <- fmt.Errorf("failed to create remove label %s patch path for nodepool %s: %w", key, name, err)
			continue
		}
		for _, node := range np.Nodes {
			nodeName := strings.TrimPrefix(node.Name, fmt.Sprintf("%s-", p.clusterID))

			kc := p.kBase
			kc.Stdout = comm.GetStdOut(p.clusterID)
			kc.Stderr = comm.GetStdErr(p.clusterID)

			p.wg.Go(func() error {
				if err := p.processLimit.Acquire(ctx, 1); err != nil {
					p.errChan <- fmt.Errorf("error while patching node, failed to acquire sempahore: %w", err)
					return nil
				}
				defer p.processLimit.Release(1)

				if err := kc.KubectlPatch("node", nodeName, patchPath, "--type", "json"); err != nil {
					logger.Err(err).Str("node", nodeName).Msgf("Failed to patch labels on node with path %s", patchPath)
					p.errChan <- fmt.Errorf("failed to remove labels on node %s with path %s: %w", nodeName, patchPath, err)
					// fallthrough
				}
				return nil
			})
		}
	}
}

func labelNodePool(
	ctx context.Context,
	logger zerolog.Logger,
	np *spec.NodePool,
	p patchData,
	additionalLabels map[string]string,
) {
	name := np.Name
	nodeLabels, err := nodes.GetAllLabels(np, nil, additionalLabels)
	if err != nil {
		p.errChan <- fmt.Errorf("failed to create labels for %s : %w", name, err)
		return
	}

	for key, value := range nodeLabels {
		patchPath, err := buildJSONPatchString("replace", "/metadata/labels/"+key, value)
		if err != nil {
			p.errChan <- fmt.Errorf("failed to create label %s patch path for nodepool %s: %w", key, name, err)
			continue
		}

		label(ctx, logger, p, patchPath, np.Nodes)
	}
}

func label(
	ctx context.Context,
	logger zerolog.Logger,
	p patchData,
	patch string,
	nodes []*spec.Node,
) {
	for _, node := range nodes {
		nodeName := strings.TrimPrefix(node.Name, fmt.Sprintf("%s-", p.clusterID))

		kc := p.kBase
		kc.Stdout = comm.GetStdOut(p.clusterID)
		kc.Stderr = comm.GetStdErr(p.clusterID)

		p.wg.Go(func() error {
			if err := p.processLimit.Acquire(ctx, 1); err != nil {
				p.errChan <- fmt.Errorf("error while patching node, failed to acquire semaphore: %w", err)
				return nil
			}
			defer p.processLimit.Release(1)

			if err := kc.KubectlPatch("node", nodeName, patch, "--type", "json"); err != nil {
				logger.Err(err).Str("node", nodeName).Msgf("Failed to patch labels on node with path %s", patch)
				p.errChan <- fmt.Errorf("error while patching one or more nodes with labels")
				// fallthrough
			}
			return nil
		})
	}
}

func removeAnnotations(
	ctx context.Context,
	logger zerolog.Logger,
	np *spec.NodePool,
	p patchData,
	toRemove []string,
) {
	name := np.Name
	for _, key := range toRemove {
		escapedKey := strings.ReplaceAll(key, "/", "~1")
		patchPath, err := buildJSONPatchString("remove", "/metadata/annotations/"+escapedKey, nil)
		if err != nil {
			p.errChan <- fmt.Errorf("failed to create remove annotation %s patch path for nodepool %s: %w", key, name, err)
			continue
		}

		for _, node := range np.Nodes {
			nodeName := strings.TrimPrefix(node.Name, fmt.Sprintf("%s-", p.clusterID))

			kc := p.kBase
			kc.Stdout = comm.GetStdOut(p.clusterID)
			kc.Stderr = comm.GetStdErr(p.clusterID)

			p.wg.Go(func() error {
				if err := p.processLimit.Acquire(ctx, 1); err != nil {
					p.errChan <- fmt.Errorf("error while patching node, failed to acquire semaphore: %w", err)
					return nil
				}
				defer p.processLimit.Release(1)

				if err := kc.KubectlPatch("node", nodeName, patchPath, "--type", "json"); err != nil {
					logger.Err(err).Str("node", nodeName).Msgf("Failed to patch annotations on node %s", nodeName)
					p.errChan <- fmt.Errorf("failed to remove annotations on node %s: %w", nodeName, err)
					// fallthrough
				}
				return nil
			})
		}
	}
}

func annotateNodePool(
	ctx context.Context,
	logger zerolog.Logger,
	p patchData,
	np *spec.NodePool,
	additionalAnnotations map[string]string,
) {
	isControl := np.IsControl
	zone := np.Zone()
	name := np.Name

	annotations := maps.Clone(np.Annotations)
	if annotations == nil {
		annotations = make(map[string]string)
	}
	for k, v := range additionalAnnotations {
		annotations[k] = v
	}

	// annotate worker nodes with provider spec name to match the storage classes
	// created in the SetupLonghorn step.
	// NOTE: the master nodes are by default set to NoSchedule, therefore we do not need to annotate them
	// If in the future, if add functionality to allow scheduling on master nodes, longhorn will need to add the annotation.
	if !isControl {
		k := "node.longhorn.io/default-node-tags"
		tags, ok := annotations[k]
		if !ok {
			tags = "[]"
		}
		var v []any
		if err := json.Unmarshal([]byte(tags), &v); err != nil {
			p.errChan <- fmt.Errorf("nodepool %s has invalid value for annotation %v, expected value to by of type array: %w", name, k, err)
			return
		}
		var found bool
		for i := range v {
			s, ok := v[i].(string)
			if !ok {
				continue
			}
			if s == zone {
				found = true
				break
			}
		}
		if !found {
			v = append(v, zone)
		}

		b, err := json.Marshal(v)
		if err != nil {
			p.errChan <- fmt.Errorf("failed to marshal modified annotations for nodepool %s: %w", name, err)
			return
		}
		annotations[k] = string(b)
	}

	patch, err := buildJSONAnnotationPatch(annotations)
	if err != nil {
		p.errChan <- fmt.Errorf("failed to create annotation for nodepool %s: %w", name, err)
		return
	}

	annotate(ctx, logger, p, patch, np.Nodes)
}

func annotate(ctx context.Context, logger zerolog.Logger, p patchData, patch string, nodes []*spec.Node) {
	for _, node := range nodes {
		nodeName := strings.TrimPrefix(node.Name, fmt.Sprintf("%s-", p.clusterID))

		kc := p.kBase
		kc.Stdout = comm.GetStdOut(p.clusterID)
		kc.Stderr = comm.GetStdErr(p.clusterID)

		p.wg.Go(func() error {
			if err := p.processLimit.Acquire(ctx, 1); err != nil {
				p.errChan <- fmt.Errorf("error while patching node, failed to acquire semaphore: %w", err)
				return nil
			}
			defer p.processLimit.Release(1)

			if err := kc.KubectlPatch("node", nodeName, patch, "--type", "merge"); err != nil {
				logger.Err(err).Str("node", nodeName).Msgf("Failed to patch annotations on node %s", nodeName)
				p.errChan <- fmt.Errorf("error while applying annotations %v for node %s: %w", patch, nodeName, err)
				// fallthrough
			}
			return nil
		})
	}
}

func removeTaints(
	ctx context.Context,
	logger zerolog.Logger,
	np *spec.NodePool,
	p patchData,
	taints []*spec.Taint,
) {
	for _, taint := range taints {
		key := taint.Key
		value := taint.Value
		effect := taint.Effect
		for _, node := range np.Nodes {
			nodeName := strings.TrimPrefix(node.Name, fmt.Sprintf("%s-", p.clusterID))

			kc := p.kBase
			kc.Stdout = comm.GetStdOut(p.clusterID)
			kc.Stderr = comm.GetStdErr(p.clusterID)

			p.wg.Go(func() error {
				if err := p.processLimit.Acquire(ctx, 1); err != nil {
					p.errChan <- fmt.Errorf("error while patching node, failed to acquire semaphore: %w", err)
					return nil
				}
				defer p.processLimit.Release(1)

				if err := kc.KubectlTaintRemove(nodeName, key, value, effect); err != nil {
					logger.Err(err).Str("node", nodeName).Msgf("Failed to remove taint %s on node %s", taint, nodeName)
					p.errChan <- fmt.Errorf("failed to remove taint %s on node %s: %w", taint, nodeName, err)
					// fallthrough
				}
				return nil
			})
		}
	}
}

func taintNodePool(
	ctx context.Context,
	logger zerolog.Logger,
	np *spec.NodePool,
	p patchData,
	additionalTaints []*spec.Taint,
) {
	name := np.Name
	taints := nodes.GetAllTaints(np, additionalTaints)

	patchPath, err := buildJSONPatchString("replace", "/spec/taints", taints)
	if err != nil {
		p.errChan <- fmt.Errorf("failed to create taints patch path for %s : %w", name, err)
		return
	}

	taint(ctx, logger, p, patchPath, np.Nodes)
}

func taint(
	ctx context.Context,
	logger zerolog.Logger,
	p patchData,
	patchPath string,
	nodes []*spec.Node,
) {
	for _, node := range nodes {
		nodeName := strings.TrimPrefix(node.Name, fmt.Sprintf("%s-", p.clusterID))

		kc := p.kBase
		kc.Stdout = comm.GetStdOut(p.clusterID)
		kc.Stderr = comm.GetStdErr(p.clusterID)

		p.wg.Go(func() error {
			if err := p.processLimit.Acquire(ctx, 1); err != nil {
				p.errChan <- fmt.Errorf("error while patching node, failed to acquire semaphore: %w", err)
				return nil
			}
			defer p.processLimit.Release(1)

			if err := kc.KubectlPatch("node", nodeName, patchPath, "--type", "json"); err != nil {
				logger.Err(err).Str("node", nodeName).Msgf("Failed to patch taints on node with path %s", patchPath)
				p.errChan <- fmt.Errorf("error while patching nodes with taints: %w", err)
				// fallthrough
			}
			return nil
		})
	}
}

func buildJSONAnnotationPatch(data map[string]string) (string, error) {
	metadata := PatchAnnotations{
		MetadataAnnotations{
			Annotations: data,
		},
	}
	jsonPatch, err := json.Marshal(metadata)
	if err != nil {
		return "", err
	}
	return string(jsonPatch), nil
}

func buildJSONPatchString(op, path string, value any) (string, error) {
	patchJson := patchJson{Op: op, Path: path, Value: value}
	b, err := json.Marshal(patchJson)
	if err != nil {
		return "", err
	}
	return "[" + string(b) + "]", nil
}
