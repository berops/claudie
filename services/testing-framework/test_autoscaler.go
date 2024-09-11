package testingframework

import (
	"context"
	"fmt"
	"time"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb/spec"
	managerclient "github.com/berops/claudie/services/manager/client"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

const (
	// Deployment which should NOT trigger scale up
	scaleUpDeploymentIgnored = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment-ignore
  labels:
    app: nginx
spec:
  replicas: 4
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
        - name: nginx
          image: nginx:1.14.2
          ports:
            - containerPort: 80
          resources:
            requests:
              memory: 8000Mi`
	// Deployment which should trigger scale up
	scaleUpDeployment = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment-success
  labels:
    app: nginx
spec:
  replicas: 4
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
        - name: nginx
          image: nginx:1.14.2
          ports:
            - containerPort: 80
          resources:
            requests:
              memory: 500Mi`
	// Time in which Autoscaler should trigger scale up
	scaleUpTimeout = 180 // 3 mins
	// Time in which Autoscaler should trigger scale down
	scaleDownTimeout = 2400 // 40 mins
)

// testAutoscaler tests the Autoscaler deployment.
func testAutoscaler(ctx context.Context, config *spec.Config) error {
	autoscaledClusters := getAutoscaledClusters(config)
	if len(autoscaledClusters) == 0 {
		return nil
	}

	manager, err := managerclient.New(&log.Logger)
	if err != nil {
		return err
	}
	defer manager.Close()

	{
		group := new(errgroup.Group)

		for _, cluster := range autoscaledClusters {
			group.Go(func() error {
				log.Info().Msgf("Deploying pods which should be ignored by autoscaler for cluster %s", cluster.ClusterInfo.Name)
				return applyDeployment(cluster, scaleUpDeploymentIgnored)
			})
		}

		if err := group.Wait(); err != nil {
			return fmt.Errorf("failed to deploy scale up deployment which should be ignored: %w", err)
		}
	}

	log.Info().Msgf("Waiting %d seconds to see if autoscaler starts the scale up [1/3]", scaleUpTimeout)
	for elapsed := 0; elapsed < scaleUpTimeout; elapsed += 30 {
		time.Sleep(30 * time.Second)

		res, err := manager.GetConfig(ctx, &managerclient.GetConfigRequest{Name: config.Name})
		if err != nil {
			return fmt.Errorf("error while retrieving config %s from DB : %w", config.Name, err)
		}

		if res.Config.Manifest.State == spec.Manifest_Scheduled {
			return fmt.Errorf("some cluster/s in config %s have been scaled up, when they should not", config.Name)
		}
	}

	log.Info().Msgf("Config %s has successfully passed autoscaling test [1/3]", config.Name)

	{
		group := new(errgroup.Group)
		// Apply scale up deployment.
		for _, cluster := range autoscaledClusters {
			group.Go(func() error {
				log.Info().Msgf("Deploying pods which should trigger scale up by autoscaler for cluster %s", cluster.ClusterInfo.Name)
				return applyDeployment(cluster, scaleUpDeployment)
			})
		}
		if err := group.Wait(); err != nil {
			return fmt.Errorf("failed to deploy scale up deployment : %w", err)
		}
	}

	// Wait before checking for changes.
	log.Info().Msgf("Waiting %d seconds to see if autoscaler starts the scale up [2/3]", scaleUpTimeout)
	scheduled := false
	for elapsed := 0; elapsed < scaleUpTimeout; elapsed += 30 {
		time.Sleep(30 * time.Second)

		res, err := manager.GetConfig(ctx, &managerclient.GetConfigRequest{Name: config.Name})
		if err != nil {
			return fmt.Errorf("error while retrieving config %s from DB : %w", config.Name, err)
		}

		scheduled = res.Config.Manifest.State == spec.Manifest_Scheduled
		if scheduled {
			break
		}
	}
	if !scheduled {
		return fmt.Errorf("some cluster/s in config %s have not been scaled up, when they should have [2/3]", config.Name)
	}

	log.Info().Msgf("Config %s has successfully passed autoscaling test [2/3]", config.Name)

	err = waitForDoneOrError(ctx, manager, testset{Config: config.Name, Set: "autoscaling", Manifest: "scale-up-test"})
	if err != nil {
		return err
	}

	// Test longhorn.
	// Get new config from DB with updated counts.
	resp, err := manager.GetConfig(ctx, &managerclient.GetConfigRequest{Name: config.Name})
	if err != nil {
		return fmt.Errorf("error while retrieving config %s from DB : %w", config.Name, err)
	}

	if err := testLonghornDeployment(ctx, resp.Config); err != nil {
		return err
	}

	{
		group := new(errgroup.Group)

		for _, cluster := range autoscaledClusters {
			group.Go(func() error {
				log.Info().Msgf("Removing pods which should trigger scale down by autoscaler for cluster %s [3/3]", cluster.ClusterInfo.Name)
				return removeDeployment(cluster, scaleUpDeployment)
			})
		}

		if err := group.Wait(); err != nil {
			return fmt.Errorf("failed to remove scale up deployment : %w", err)
		}
	}

	log.Info().Msgf("Waiting %d seconds to let autoscaler start the scale down [3/3]", scaleDownTimeout)
	scheduled = false
	for elapsed := 0; elapsed < scaleDownTimeout; elapsed += 30 {
		time.Sleep(30 * time.Second)

		res, err := manager.GetConfig(ctx, &managerclient.GetConfigRequest{Name: config.Name})
		if err != nil {
			return fmt.Errorf("error while retrieving config %s from DB : %w", config.Name, err)
		}

		scheduled = res.Config.Manifest.State == spec.Manifest_Scheduled
		if scheduled {
			break
		}
	}
	if !scheduled {
		return fmt.Errorf("some cluster/s in config %s have not been scaled down, when they should have [3/3]", config.Name)
	}

	log.Info().Msgf("Config %s has successfully passed autoscaling test [3/3]", config.Name)

	err = waitForDoneOrError(ctx, manager, testset{Config: config.Name, Set: "autoscaling", Manifest: "scale-down-test"})
	if err != nil {
		return err
	}

	// Get new config from DB with updated counts.
	resp, err = manager.GetConfig(ctx, &managerclient.GetConfigRequest{Name: config.Name})
	if err != nil {
		return fmt.Errorf("error while retrieving config %s from DB : %w", config.Name, err)
	}

	return testLonghornDeployment(ctx, resp.Config)
}

// applyDeployment applies specified deployment into specified cluster.
func applyDeployment(c *spec.K8Scluster, deployment string) error {
	kc := kubectl.Kubectl{Kubeconfig: c.Kubeconfig, MaxKubectlRetries: 5}
	if log.Logger.GetLevel() == zerolog.DebugLevel {
		prefix := utils.GetClusterID(c.ClusterInfo)
		kc.Stdout = comm.GetStdOut(prefix)
		kc.Stderr = comm.GetStdErr(prefix)
	}
	if err := kc.KubectlApplyString(deployment, "-n", "default"); err != nil {
		return fmt.Errorf("failed to apply deployment on cluster %s : %w", c.ClusterInfo.Name, err)
	}
	return nil
}

// removeDeployment deletes specified deployment from specified cluster.
func removeDeployment(c *spec.K8Scluster, deployment string) error {
	kc := kubectl.Kubectl{Kubeconfig: c.Kubeconfig, MaxKubectlRetries: 5}
	if log.Logger.GetLevel() == zerolog.DebugLevel {
		prefix := utils.GetClusterID(c.ClusterInfo)
		kc.Stdout = comm.GetStdOut(prefix)
		kc.Stderr = comm.GetStdErr(prefix)
	}
	if err := kc.KubectlDeleteString(deployment, "-n", "default"); err != nil {
		return fmt.Errorf("failed to remove deployment on cluster %s : %w", c.ClusterInfo.Name, err)
	}
	return nil
}
