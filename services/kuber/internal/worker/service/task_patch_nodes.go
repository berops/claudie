package service

import (
	"slices"

	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/kuber/internal/worker/service/internal/nodes"
	"github.com/rs/zerolog"

	"golang.org/x/sync/semaphore"
)

func PatchNodes(logger zerolog.Logger, processlimit *semaphore.Weighted, workersLimit int, tracker Tracker) {
	var k8s *spec.K8Scluster
	var patch *spec.Update_KuberPatchNodes

	switch do := tracker.Task.Do.(type) {
	case *spec.Task_Create:
		k8s = do.Create.K8S
		for _, np := range k8s.ClusterInfo.NodePools {
			patch.Add = &spec.Update_KuberPatchNodes_AddBatch{
				Taints: map[string]*spec.Update_KuberPatchNodes_ListOfTaints{
					np.Name: &spec.Update_KuberPatchNodes_ListOfTaints{
						Taints: np.Taints,
					},
				},
				Labels: map[string]*spec.Update_KuberPatchNodes_MapOfLabels{
					np.Name: &spec.Update_KuberPatchNodes_MapOfLabels{
						Labels: np.Labels,
					},
				},
				Annotations: map[string]*spec.Update_KuberPatchNodes_MapOfAnnotations{
					np.Name: &spec.Update_KuberPatchNodes_MapOfAnnotations{
						Annotations: np.Annotations,
					},
				},
			}
		}
	case *spec.Task_Update:
		delta, ok := do.Update.Delta.(*spec.Update_KpatchNodes)
		if !ok {
			logger.
				Warn().
				Msgf("Received update task %T while wanting to patch nodes, assuming it was mischeduled, ignoring", do.Update.Delta)
			return
		}

		k8s = do.Update.State.K8S
		patch = delta.KpatchNodes
	default:
		logger.
			Warn().
			Msgf("Received task %T while wanting to patch nodes, assuming it was mischeduled, ignoring", tracker.Task.Do)
		return
	}

	logger.Info().Msg("Patching nodes")

	if err := nodes.Patch(logger, patch, k8s, processlimit, workersLimit); err != nil {
		logger.Err(err).Msg("Failed to patch nodes")
		tracker.Diagnostics.Push(err)
		return
	}

	removeAnnotationsLabelsTaints(k8s, patch.Remove)
	updateExistingAnnotationsLabelsTaints(k8s, patch.Add)

	u := tracker.Result.Update()
	u.Kubernetes(k8s)
	u.Commit()

	logger.Info().Msg("Nodes were successfully patched")
}

func removeAnnotationsLabelsTaints(k8s *spec.K8Scluster, remove *spec.Update_KuberPatchNodes_RemoveBatch) {
	for _, np := range k8s.ClusterInfo.NodePools {
		if v, ok := remove.Annotations[np.Name]; ok {
			for _, k := range v.Annotations {
				delete(np.Annotations, k)
			}
		}

		if v, ok := remove.Labels[np.Name]; ok {
			for _, k := range v.Labels {
				delete(np.Labels, k)
			}
		}

		if v, ok := remove.Taints[np.Name]; ok {
			np.Taints = slices.DeleteFunc(np.Taints, func(t *spec.Taint) bool {
				for _, k := range v.Taints {
					match := k.Key == t.Key
					match = match && k.Value == t.Value
					match = match && k.Effect == t.Effect
					if match {
						return true
					}
				}
				return false
			})
		}
	}
}

func updateExistingAnnotationsLabelsTaints(k8s *spec.K8Scluster, add *spec.Update_KuberPatchNodes_AddBatch) {
	for _, np := range k8s.ClusterInfo.NodePools {
		if m, ok := add.Annotations[np.Name]; ok {
			for k, v := range m.Annotations {
				np.Annotations[k] = v
			}
		}

		if m, ok := add.Labels[np.Name]; ok {
			for k, v := range m.Labels {
				np.Labels[k] = v
			}
		}

		if m, ok := add.Taints[np.Name]; ok {
			np.Taints = append(np.Taints, m.Taints...)
		}
	}
}
