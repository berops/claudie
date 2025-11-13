package service

// func reconciliate(current, desired *spec.Clusters) *spec.TaskEvent {
// 	if delta := reconciliateK8s(current.K8S, desired.K8S); delta != nil {
// 		return delta
// 	}

// 	if delta := reconciliateLBs(current.LoadBalancers, desired.LoadBalancers); delta != nil {
// 		return delta
// 	}

// 	return nil
// }

// func reconciliateK8s(current, desired *spec.K8Scluster) *spec.TaskEvent {
// 	currentDynamic, currentStatic := NodePoolsView(current)
// 	desiredDynamic, desiredStatic := NodePoolsView(desired)

// 	// Node names are transferred over from current state based on the public IP.
// 	// Thus, at this point we can figure out based on nodes names which were deleted/added
// 	// see existing_state.go:transferStaticNodes
// 	staticDiff := NodePoolsDiff(currentStatic, desiredStatic)
// 	dynamicDiff := NodePoolsDiff(currentDynamic, desiredDynamic)

// 	_ = staticDiff
// 	_ = dynamicDiff

// 	return nil
// }

// func reconciliateLBs(current, desired *spec.LoadBalancers) *spec.TaskEvent {
// 	return nil
// }
