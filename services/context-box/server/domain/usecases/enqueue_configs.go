package usecases

//import (
//	"fmt"
//	"github.com/berops/claudie/proto/pb/spec"
//	"github.com/berops/claudie/services/context-box/server/domain/ports"
//	"github.com/berops/claudie/services/context-box/server/domain/usecases/metrics"
//	"github.com/berops/claudie/services/context-box/server/utils"
//	"github.com/rs/zerolog/log"
//)
//
//const (
//	// defaultBuildTime is the time in which a single task has to finish otherwise
//	// it will be scheduled to the task queue again.
//	defaultBuildTime = 450
//
//	// default TTL for an element to be in the scheduler queue
//	defaultSchedulerTTL = 5
//)
//
//// EnqueueConfigs is a driver for enqueueConfigs function
//func (u *Usecases) EnqueueConfigs() error {
//	if err := u.enqueueConfigs(); err != nil {
//		return fmt.Errorf("error while enqueuing configs: %w", err)
//	}
//
//	if !u.schedulerQueue.CompareElementNameList(u.schedulerLogQueue) {
//		log.Info().Msgf("Scheduler queue content changed to: %v", u.schedulerQueue.IDs())
//	}
//	u.schedulerLogQueue = u.schedulerQueue.IDs()
//
//	if !u.tasksQueue.CompareElementNameList(u.taskLogQueue) {
//		log.Info().Msgf("Tasks queue content changed to: %v", u.tasksQueue.IDs())
//	}
//	u.taskLogQueue = u.tasksQueue.IDs()
//
//	return nil
//}
//
//// enqueueConfigs checks all configs, decides if they should be enqueued or not and updates their TTLs
//func (u *Usecases) enqueueConfigs() error {
//	cfgs, err := getConfigInfosFromDB(u.DB)
//	if err != nil {
//		return err
//	}
//
//	for _, cfg := range cfgs {
//		if u.schedulerQueue.Contains(cfg) {
//			continue
//		}
//
//		// If the DsChecksum is not set yet, the config was not processed by scheduler.
//		if !utils.Equal(cfg.DsChecksum, cfg.MsChecksum) {
//			if cfg.SchedulerTTL <= 0 {
//				if !cfg.HasAnyError() || cfg.CanBeScheduledForDeletion() {
//					if err := u.DB.UpdateSchedulerTTL(cfg.Name, defaultSchedulerTTL); err != nil {
//						return err
//					}
//					u.schedulerQueue.Enqueue(cfg)
//					metrics.InputManifestsEnqueuedScheduler.Inc()
//				}
//			} else if !cfg.HasAnyError() {
//				cfg.SchedulerTTL -= 1
//				if err := u.DB.UpdateSchedulerTTL(cfg.Name, cfg.SchedulerTTL); err != nil {
//					return err
//				}
//			}
//		}
//
//		for cluster, tasks := range cfg.Events {
//			if len(tasks.Events) == 0 {
//				continue // no work for this cluster.
//			}
//
//			nextTask := tasks.Events[0]
//			if u.tasksQueue.Contains(nextTask) {
//				continue
//			}
//
//			// TODO: rename ?
//			if tasks.Ttl <= 0 {
//				if !cfg.HasAnyError() || cfg.CanBeScheduledForDeletion() {
//					if err := u.DB.UpdateTaskLease(cfg.Name, cluster, defaultBuildTime); err != nil {
//						return err
//					}
//					u.tasksQueue.Enqueue(nextTask)
//				}
//			} else if !cfg.HasDestroyError() {
//				tasks.Ttl -= 1
//				if err := u.DB.UpdateTaskLease(cfg.Name, cluster, tasks.Ttl); err != nil {
//					return err
//				}
//			}
//		}
//	}
//
//	return nil
//}
//
//// Fetches all configAsBSON from MongoDB and converts each configAsBSON to ConfigInfo
//// Then returns the list
//func getConfigInfosFromDB(mongoDB ports.DBPort) ([]*spec.Config, error) {
//	configs, err := mongoDB.GetAllConfigs()
//	if err != nil {
//		return nil, err
//	}
//
//	return configs, nil
//}
