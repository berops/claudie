package service

import (
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/kuber/internal/worker/service/internal/nodes"
	"github.com/rs/zerolog"
	"golang.org/x/sync/semaphore"
)

func PatchNodes(logger zerolog.Logger, processlimit *semaphore.Weighted, workersLimit int, tracker Tracker) {
	update, ok := tracker.Task.Do.(*spec.TaskV2_Update)
	if !ok {
		logger.
			Warn().
			Msgf("Received task %T while wanting to patch nodes, assuming it was mischeduled, ignoring", tracker.Task.Do)
		return
	}

	delta, ok := update.Update.Delta.(*spec.UpdateV2_KpatchNodes)
	if !ok {
		logger.
			Warn().
			Msgf("Received update task %T while wanting to patch nodes, assuming it was mischeduled, ignoring", update.Update.Delta)
		return
	}

	logger.Info().Msg("Patching nodes")

	patcher := nodes.NewPatcher(logger, update.Update.State.K8S, delta.KpatchNodes, processlimit, workersLimit)
	if err := patcher.Wait(); err != nil {
		logger.Err(err).Msg("Failed to patch nodes")
		tracker.Diagnostics.Push(err)
		return
	}

	logger.Info().Msg("Nodes were successfully patched")
}
