package usecases

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
)

func (u *Usecases) WatchConfigs() {
	ticker := time.NewTicker(10 * time.Second)

	configs, err := u.ContextBox.GetAllConfigs()
	if err != nil {
		log.Err(err).Msgf("Failed to retrieve configs from context-box")
	}

	for _, config := range configs {
		for k8sClusterName, workflow := range config.State {
			if workflow.Status == pb.Workflow_ERROR || workflow.Status == pb.Workflow_DONE {
				u.inProgress.Store(k8sClusterName, config)
			}
		}
	}

	log.Info().Msgf("Frontend is ready to watch input manifest statuses")
	for {
		select {
		case <-u.Context.Done():
			return
		case <-ticker.C:
			{
				configs, err = u.ContextBox.GetAllConfigs()
				if err != nil {
					log.Err(err).Msgf("Failed to retrieve configs from context-box")
					break
				}

				// Find configs that have been deleted from the DB. This indicates that the infra from the config has been removed.
				u.inProgress.Range(func(key, value any) bool {
					cluster := key.(string)
					cfg := value.(*pb.Config)

					for _, config := range configs {
						if config.Name == cfg.Name {
							return true // continue
						}
					}

					u.inProgress.Delete(cluster)
					
					log.Info().Str("project", cfg.Name).Str("cluster", cluster).Msgf("Cluster has been deleted")
					return true
				})

				// Print the state of the configs that are in the workflow (reconcile state)
				for _, config := range configs {
					for cluster, workflow := range config.State {
						logger := utils.CreateLoggerWithProjectAndClusterName(config.Name, cluster)

						_, ok := u.inProgress.Load(cluster)
						// Config had errors while building
						if workflow.Status == pb.Workflow_ERROR {
							if ok {
								u.inProgress.Delete(cluster)

								
								logger.Err(errors.New(workflow.Description)).Msgf("Workflow failed")
							}
							continue
						}
						// Config has been build
						if workflow.Status == pb.Workflow_DONE {
							if ok {
								u.inProgress.Delete(cluster)

								logger.Info().Msgf("Workflow finished")
							}
							continue
						}

						u.inProgress.Store(cluster, config)

						stringBuilder := new(strings.Builder)
						stringBuilder.WriteString(
							fmt.Sprintf("Cluster currently in stage %s with status %s", workflow.Stage.String(), workflow.Status.String()),
						)
						if workflow.Description != "" {
							stringBuilder.WriteString(fmt.Sprintf(" %s", strings.TrimSpace(workflow.Description)))
						}

						logger.Info().Msgf(stringBuilder.String())
					}
				}
			}
		}
	}
}
