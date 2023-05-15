package usecases

import (
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/berops/claudie/proto/pb"
)

func (u *Usecases) WatchConfigs() {
	ticker := time.NewTicker(10 * time.Second)

	configs, err := u.ContextBox.GetAllConfigs()
	if err != nil {
		log.Error().Msgf("failed to retrieve configs from contextbox: %s", err)
	}

	for _, config := range configs {
		for k8sClusterName, workflow := range config.State {
			if workflow.Status == pb.Workflow_ERROR || workflow.Status == pb.Workflow_DONE {
				u.inProgress.Store(k8sClusterName, config)
			}
		}
	}

	for {
		select {
		case <-u.Context.Done():
			return
		case <-ticker.C:
			{
				configs, err = u.ContextBox.GetAllConfigs()
				if err != nil {
					log.Error().Msgf("Failed to retrieve configs from context-box microservice: %s", err)
					break
				}

				// Find configs that have been deleted from the DB.
				u.inProgress.Range(func(key, value any) bool {
					cluster := key.(string)
					cfg := value.(*pb.Config)

					for _, config := range configs {
						if config.Name == cfg.Name {
							return true // continue
						}
					}

					u.inProgress.Delete(cluster)
					log.Info().Msgf("Config: %s - cluster %s has been deleted", cfg.Name, cluster)
					return true
				})

				for _, config := range configs {
					for cluster, workflow := range config.State {
						_, ok := u.inProgress.Load(cluster)
						if workflow.Status == pb.Workflow_ERROR {
							if ok {
								u.inProgress.Delete(cluster)
								log.Error().Msgf("Workflow failed for cluster %s:%s", cluster, workflow.Description)
							}
							continue
						}
						if workflow.Status == pb.Workflow_DONE {
							if ok {
								u.inProgress.Delete(cluster)
								log.Info().Msgf("Workflow finished for cluster %s", cluster)
							}
							continue
						}

						u.inProgress.Store(cluster, config)

						stringBuilder := new(strings.Builder)
						stringBuilder.WriteString(
							fmt.Sprintf("Cluster %s currently in stage %s with status %s", cluster, workflow.Stage.String(), workflow.Status.String()),
						)
						if workflow.Description != "" {
							stringBuilder.WriteString(fmt.Sprintf(" %s", strings.TrimSpace(workflow.Description)))
						}
						log.Info().Msgf("Config: %s - %s", config.Name, stringBuilder.String())
					}
				}
			}
		}
	}
}
