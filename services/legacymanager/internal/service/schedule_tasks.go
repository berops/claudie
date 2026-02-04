package service

// func scheduleTasks(scheduled *store.Config) (ScheduleResult, error) {
// 	for cluster, state := range scheduledGRPC.Clusters {
// 		logger := loggerutils.WithProjectAndCluster(scheduledGRPC.Name, cluster)

// 		var events []*spec.TaskEvent
// 		switch {
// 		default:
// 			if state.State.Status == spec.Workflow_ERROR {
// 				if tasks := state.Events.Events; len(tasks) != 0 && tasks[0].OnError.Do != nil {
// 					task := tasks[0]

// 					switch s := task.OnError.Do.(type) {
// 					case *spec.Retry_Repeat_:
// 						events = tasks

// 						if s.Repeat.Kind == spec.Retry_Repeat_EXPONENTIAL {
// 							if s.Repeat.RetryAfter > 0 {
// 								s.Repeat.RetryAfter--
// 								result = NotReady
// 								break
// 							}

// 							s.Repeat.CurrentTick <<= 1
// 							if s.Repeat.CurrentTick >= s.Repeat.StopAfter {
// 								// final retry before error-ing out.
// 								result = FinalRetry
// 								task.OnError.Do = nil
// 								break
// 							}

// 							s.Repeat.RetryAfter = s.Repeat.CurrentTick
// 						}

// 						result = Reschedule
// 						logger.Debug().Msgf("rescheduled for a retry of previously failed task with ID %q.", task.Id)
// 					case *spec.Retry_Rollback_:
// 						result = Reschedule
// 						events = s.Rollback.Tasks
// 						logger.Debug().Msgf("rescheduled for a rollback with task ID %q of previous failed task with ID %q.", events[0].Id, task.Id)
// 					default:
// 						result = NoReschedule
// 						logger.Debug().Msgf("has not been rescheduled for a retry on failure")
// 					}

// 					if result == Reschedule || result == NotReady || result == FinalRetry {
// 						break
// 					}
// 				}
// 			}
// 		}
// 	}
// }
