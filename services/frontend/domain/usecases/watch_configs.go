package usecases

import (
	"errors"
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
		log.Err(err).Msgf("Failed to retrieve configs from context-box")
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
		case <-u.Done:
			return
		case <-ticker.C:
			{
				configs, err = u.ContextBox.GetAllConfigs()
				if err != nil {
					log.Err(err).Msgf("Failed to retrieve configs from context-box")
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
					log.Info().
						Str("manifest", cfg.Name).
						Str("cluster", cluster).
						Msgf("Cluster has been deleted")
					return true
				})

				for _, config := range configs {
					for cluster, workflow := range config.State {
						_, ok := u.inProgress.Load(cluster)
						if workflow.Status == pb.Workflow_ERROR {
							if ok {
								u.inProgress.Delete(cluster)

								log.Err(errors.New(workflow.Description)).
									Str("cluster", cluster).
									Msgf("Workflow failed")
							}
							continue
						}
						if workflow.Status == pb.Workflow_DONE {
							if ok {
								u.inProgress.Delete(cluster)

								log.Info().
									Str("cluster", cluster).
									Msgf("Workflow finished")
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

						log.Info().
							Str("cluster", cluster).
							Str("manifest", config.Name).
							Msgf(stringBuilder.String())
					}
				}
			}
		}
	}
}
