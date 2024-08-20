package spec

//func (cfg *Config) ID() string { return cfg.Name }
//
//// HasAnyError returns true if any cluster errored while building or destroying.
//func (cfg *Config) HasAnyError() bool {
//	for _, v := range cfg.State {
//		if v.Status == Workflow_ERROR {
//			return true
//		}
//	}
//	return false
//}
//
//// HasDestroyError returns true if error occurred in any of the clusters while getting destroyed.
//func (cfg *Config) HasDestroyError() bool {
//	for _, v := range cfg.State {
//		inDestroyStage := v.Stage == Workflow_DESTROY_TERRAFORMER || v.Stage == Workflow_DESTROY_KUBER || v.Stage == Workflow_DELETE_NODES
//		if v.Status == Workflow_ERROR && inDestroyStage {
//			return true
//		}
//	}
//	return false
//}
//
//// CanBeScheduledForDeletion returns true if config should be pushed onto any queue due to deletion.
//// This ignores the build errors, as we want to remove infrastructure if secret was deleted,
//// However, respects error from destroy workflow, as we do not want to retry indefinitely.
//func (cfg *Config) CanBeScheduledForDeletion() bool {
//	// Ignore as deletion already errored out
//	if cfg.HasDestroyError() {
//		return false
//	}
//
//	// Scheduler queue
//	if cfg.MsChecksum == nil && cfg.DsChecksum != nil {
//		return true
//	}
//
//	// Tasks Queue
//	if cfg.MsChecksum == nil && cfg.DsChecksum == nil && cfg.CsChecksum != nil {
//		return true
//	}
//
//	return false
//}
