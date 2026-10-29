package service

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog"

	"golang.org/x/sync/semaphore"
)

const nodeSetupPlaybook = "../../ansible-playbooks/node-setup.yml"

// SetupNode installs the node requirements (packages, kubernetes binaries, limits) and
// the wireguard VPN across all of the Loadbalancer and kubernetes nodes.
func SetupNode(
	logger zerolog.Logger,
	projectName string,
	processLimit *semaphore.Weighted,
	tracker Tracker,
) {
	logger.Info().Msg("Setting up nodes")

	k8s, lbs, vi, ok := vpnTargets(logger, tracker)
	if !ok {
		return
	}

	// The node requirements of the kubernetes nodes include the Kubernetes packages of this version,
	// which kubeone would otherwise install one node group at a time (control plane first, then
	// workers) during `kubeone apply` in kube-eleven.
	var flags string
	if v := strings.TrimPrefix(strings.TrimSpace(k8s.Kubernetes), "v"); v != "" {
		flags = fmt.Sprintf("--extra-vars \"kubernetes_version=%s\"", v)
	}

	if err := runPlaybookOnVPNNodes(k8s.ClusterInfo.Id(), vi, nodeSetupPlaybook, flags, processLimit); err != nil {
		logger.Err(err).Msg("Failed to set up nodes")
		tracker.Diagnostics.Push(err)
		return
	}

	update := tracker.Result.Update()
	update.Kubernetes(k8s)
	update.Loadbalancers(lbs...)
	update.Commit()

	logger.Info().Msg("Finished setting up nodes")
}
