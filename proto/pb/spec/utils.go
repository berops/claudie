package spec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"
)

// TODO: remove any unsued functions after the refactor.
// MergeTargetPools takes the target pools from the other role
// and adds them to this role, ignoring duplicates.

// ErrCloudflareAPIForbidden is returned when the response to the endpoint of cloudflare returns code 403,
// which means that the endpoint cannot be reached with the current account-id/token pair.
var ErrCloudflareAPIForbidden = errors.New("token/account-id pair with the cloudflare provider does not have acces for the necessary API")

// Id returns the ID of the cluster.
func (c *ClusterInfo) Id() string {
	if c == nil {
		return ""
	}
	return fmt.Sprintf("%s-%s", c.Name, c.Hash)
}

// HasApiRole checks whether the LB has a role with port 6443.
func (c *LBcluster) HasApiRole() bool {
	if c == nil {
		return false
	}

	for _, role := range c.Roles {
		if role.RoleType == RoleType_ApiServer {
			return true
		}
	}

	return false
}

// IsApiEndpoint  checks whether the LB is selected as the API endpoint.
func (c *LBcluster) IsApiEndpoint() bool {
	if c == nil {
		return false
	}
	return c.HasApiRole() && c.UsedApiEndpoint
}

// EndpointNode searches for a node with type ApiEndpoint.
func (n *NodePool) EndpointNode() *Node {
	if n == nil {
		return nil
	}

	for _, node := range n.Nodes {
		if node.NodeType == NodeType_apiEndpoint {
			return node
		}
	}

	return nil
}

// Credentials extract the key for the provider to be used within terraform.
func (pr *Provider) Credentials() string {
	if pr == nil {
		return ""
	}

	switch p := pr.ProviderType.(type) {
	case *Provider_Gcp:
		return p.Gcp.Key
	case *Provider_Hetzner:
		return p.Hetzner.Token
	case *Provider_Hetznerdns:
		return p.Hetznerdns.Token
	case *Provider_Oci:
		return p.Oci.PrivateKey
	case *Provider_Aws:
		return p.Aws.SecretKey
	case *Provider_Azure:
		return p.Azure.ClientSecret
	case *Provider_Cloudflare:
		return p.Cloudflare.Token
	case *Provider_Genesiscloud:
		return p.Genesiscloud.Token
	case *Provider_Openstack:
		return p.Openstack.ApplicationCredentialSecret
	default:
		panic(fmt.Sprintf("unexpected type %T", pr.ProviderType))
	}
}

// MustExtractTargetPath returns the target path of the external template repository.
// If the URL of the repository is invalid this functions panics.
// The target path is the path where the templates should be downloaded on the local
// filesystem.
func (r *TemplateRepository) MustExtractTargetPath() string {
	if r == nil {
		return ""
	}

	u, err := url.Parse(r.Repository)
	if err != nil {
		panic(err)
	}

	return filepath.Join(
		u.Hostname(),
		u.Path,
		r.CommitHash,
		r.Path,
	)
}

func (n *NodePool) Zone() string {
	var sn string

	switch {
	case n.GetDynamicNodePool() != nil:
		sn = n.GetDynamicNodePool().Provider.SpecName
	case n.GetStaticNodePool() != nil:
		sn = StaticNodepoolInfo_STATIC_PROVIDER.String()
	default:
		panic("unsupported nodepool type")
	}

	if sn == "" {
		panic("no zone specified")
	}

	return fmt.Sprintf("%s-zone", sn)
}

// GetSubscription checks if the Cloudflare account has a Load Balancing subscription.
func (x *CloudflareProvider) GetSubscription() (bool, error) {
	// the number of retries before returning an error on trying to
	// communicate with the cloudflare API.
	const retries = 3

	var subscriptions struct {
		Result []struct {
			ID      string `json:"id"`
			Product struct {
				Name string `json:"name"`
			} `json:"product"`
		} `json:"result"`
		Success bool `json:"success"`
	}

	escapedAccountID := url.PathEscape(x.AccountID)
	urlSubscriptions := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/subscriptions", escapedAccountID)

	var response []byte
	var err error

	// The api seems to fail sometimes, add more checks with a exponential backoff before giving up.
	for i := range retries {
		response, err = getCloudflareAPIResponse(urlSubscriptions, x.Token)
		if err != nil {
			if errors.Is(err, ErrCloudflareAPIForbidden) {
				return false, nil
			}
			time.Sleep((1 << i) * time.Second)
			continue
		}
		break
	}

	if err != nil {
		return false, fmt.Errorf("error while getting cloudflare api response for 'accounts/subscriptions', after %v retries: %w", retries, err)
	}

	if err := json.Unmarshal(response, &subscriptions); err != nil {
		return false, fmt.Errorf("failed to parse JSON: %w", err)
	}

	for _, subscription := range subscriptions.Result {
		if subscription.Product.Name == "prod_load_balancing" && subscriptions.Success {
			return true, nil
		}
	}
	return false, fmt.Errorf("subscription for Load Balancing not found")
}

func getCloudflareAPIResponse(url string, apiToken string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("could not create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		return nil, ErrCloudflareAPIForbidden
	}

	// nolint
	if !(resp.StatusCode >= 200 && resp.StatusCode < 300) {
		return nil, fmt.Errorf("response with status code %v: %v", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	return body, nil
}

// Consumes the [TaskResult_Clear] for the task.
func (te *Task) ConsumeClearResult(result *TaskResult_Clear) {
	var k8s **K8Scluster
	var lbs *[]*LBcluster

	switch task := te.Do.(type) {
	case *Task_Create:
		k8s = &task.Create.K8S
		lbs = &task.Create.LoadBalancers
	case *Task_Delete:
		k8s = &task.Delete.K8S
		lbs = &task.Delete.LoadBalancers
	case *Task_Update:
		k8s = &task.Update.State.K8S
		lbs = &task.Update.State.LoadBalancers
	default:
		return
	}

	if result.Clear.K8S != nil && *result.Clear.K8S {
		*k8s = nil
		*lbs = nil
		return
	}

	lbFilter := func(lb *LBcluster) bool {
		return slices.Contains(result.Clear.LoadBalancersIDs, lb.GetClusterInfo().Id())
	}
	*lbs = slices.DeleteFunc(*lbs, lbFilter)
}

// Consumes the [TaskResult_Update] for the task.
func (te *Task) ConsumeUpdateResult(result *TaskResult_Update) error {
	var k8s **K8Scluster
	var lbs *[]*LBcluster

	switch task := te.Do.(type) {
	case *Task_Create:
		k8s = &task.Create.K8S
		lbs = &task.Create.LoadBalancers
	case *Task_Delete:
		k8s = &task.Delete.K8S
		lbs = &task.Delete.LoadBalancers
	case *Task_Update:
		k8s = &task.Update.State.K8S
		lbs = &task.Update.State.LoadBalancers
	default:
		return nil
	}

	id := (*k8s).ClusterInfo.Id()
	name := (*k8s).ClusterInfo.Name

	// Consume delete updates, if any.
	if update := te.GetUpdate(); update != nil {
		switch delta := update.Delta.(type) {
		case *Update_KDeleteNodes:
			idx := slices.IndexFunc((*k8s).ClusterInfo.NodePools, func(n *NodePool) bool {
				return n.Name == delta.KDeleteNodes.Nodepool
			})
			if idx < 0 {
				// Under normal circumstances this should never happen, this signals
				// either malformed/corrupted data and/or mistake in the schedule of
				// tasks. Thus rather return an error than continiung with the merge.
				return fmt.Errorf("can't update cluster %q received update result with invalid deleted nodepool %q", id, delta.KDeleteNodes.Nodepool)
			}

			affected := (*k8s).ClusterInfo.NodePools[idx]

			if delta.KDeleteNodes.WithNodePool {
				// Below, with the replacement of the kuberentes
				// cluster it should no longer reference this nodepool
				// and this should be the only owner of it afterwards.
				update.Delta = &Update_DeletedK8SNodes_{
					DeletedK8SNodes: &Update_DeletedK8SNodes{
						Kind: &Update_DeletedK8SNodes_Whole{
							Whole: &Update_DeletedK8SNodes_WholeNodePool{
								Nodepool: affected,
							},
						},
					},
				}
			} else {
				// Below, with the replacement of the kubernetes
				// cluster it should no longer reference these nodes
				// and this should be the only owner of them afterwards.
				d := &Update_DeletedK8SNodes_Partial_{
					Partial: &Update_DeletedK8SNodes_Partial{
						Nodepool:       delta.KDeleteNodes.Nodepool,
						Nodes:          []*Node{},
						StaticNodeKeys: map[string]string{},
					},
				}

				for _, n := range affected.Nodes {
					if slices.Contains(delta.KDeleteNodes.Nodes, n.Name) {
						d.Partial.Nodes = append(d.Partial.Nodes, n)
					}
				}

				if stt := affected.GetStaticNodePool(); stt != nil {
					for _, n := range d.Partial.Nodes {
						key := n.Public
						d.Partial.StaticNodeKeys[key] = stt.NodeKeys[key]
					}
				}

				update.Delta = &Update_DeletedK8SNodes_{
					DeletedK8SNodes: &Update_DeletedK8SNodes{
						Kind: d,
					},
				}
			}
		case *Update_TfDeleteLoadBalancerNodes:
			handle := delta.TfDeleteLoadBalancerNodes.Handle
			lbi := slices.IndexFunc(*lbs, func(lb *LBcluster) bool {
				return lb.ClusterInfo.Id() == handle
			})
			if lbi < 0 {
				// Under normal circumstances this should never happen, this signals
				// either malformed/corrupted data and/or mistake in the schedule of
				// tasks. Thus rather return an error than continiung with the merge.
				return fmt.Errorf("can't update loadbalancer %q received update result with invalid loadbalancer id", id)
			}

			lb := (*lbs)[lbi]

			npi := slices.IndexFunc(lb.ClusterInfo.NodePools, func(n *NodePool) bool {
				return n.Name == delta.TfDeleteLoadBalancerNodes.Nodepool
			})
			if npi < 0 {
				// Under normal circumstances this should never happen, this signals
				// either malformed/corrupted data and/or mistake in the schedule of
				// tasks. Thus rather return an error than continiung with the merge.
				return fmt.Errorf("can't update loadbalancer %q received update result with invalid deleted nodepool %q", id, delta.TfDeleteLoadBalancerNodes.Nodepool)
			}

			affected := lb.ClusterInfo.NodePools[npi]

			if delta.TfDeleteLoadBalancerNodes.WithNodePool {
				// Below, with the replacement of the loadbalancer
				// clsuter it should no longer reference this nodepool
				// and this should be the only owner of it afterwards.
				update.Delta = &Update_DeletedLoadBalancerNodes_{
					DeletedLoadBalancerNodes: &Update_DeletedLoadBalancerNodes{
						Handle: handle,
						Kind: &Update_DeletedLoadBalancerNodes_Whole{
							Whole: &Update_DeletedLoadBalancerNodes_WholeNodePool{
								Nodepool: affected,
							},
						},
					},
				}
			} else {
				// Below, with the replacement of the loadbalancer cluster
				// it should no longer reference these nodes and this should
				// be the only owner of them afterwards.
				d := &Update_DeletedLoadBalancerNodes_Partial_{
					Partial: &Update_DeletedLoadBalancerNodes_Partial{
						Nodepool:       delta.TfDeleteLoadBalancerNodes.Nodepool,
						Nodes:          []*Node{},
						StaticNodeKeys: map[string]string{},
					},
				}

				for _, n := range affected.Nodes {
					if slices.Contains(delta.TfDeleteLoadBalancerNodes.Nodes, n.Name) {
						d.Partial.Nodes = append(d.Partial.Nodes, n)
					}
				}

				if stt := affected.GetStaticNodePool(); stt != nil {
					for _, n := range d.Partial.Nodes {
						key := n.Public
						d.Partial.StaticNodeKeys[key] = stt.NodeKeys[key]
					}
				}

				update.Delta = &Update_DeletedLoadBalancerNodes_{
					DeletedLoadBalancerNodes: &Update_DeletedLoadBalancerNodes{
						Handle: handle,
						Kind:   d,
					},
				}
			}
		}
	}

	if k := result.Update.K8S; k != nil {
		if gotName := k.GetClusterInfo().Id(); gotName != id {
			// Under normal circumstances this should never happen, this signals either
			// malformed/corrupted data and/or mistake in the scheduling of tasks.
			// Thus return an error rather than continuing with the merge.
			return fmt.Errorf("can't update cluster %q with received cluster %q", id, gotName)
		}
		(*k8s) = k
		result.Update.K8S = nil
	}

	var toUpdate LoadBalancers
	for _, lb := range result.Update.LoadBalancers.Clusters {
		toUpdate.Clusters = append(toUpdate.Clusters, lb)
	}
	result.Update.LoadBalancers.Clusters = nil

	toUpdate.Clusters = slices.DeleteFunc(toUpdate.Clusters, func(lb *LBcluster) bool {
		return lb.TargetedK8S != name
	})

	// update existing ones.
	for i := range *lbs {
		lb := (*lbs)[i].ClusterInfo.Id()
		for j := range toUpdate.Clusters {
			if update := toUpdate.Clusters[j].ClusterInfo.Id(); lb == update {
				(*lbs)[i] = toUpdate.Clusters[j]
				toUpdate.Clusters = slices.Delete(toUpdate.Clusters, j, j+1)
				break
			}
		}
	}

	// add new ones.
	*lbs = append(*lbs, toUpdate.Clusters...)

	// Consume replace updates, if any.
	if update := te.GetUpdate(); update != nil {
		switch delta := update.Delta.(type) {
		case *Update_AnsReplaceProxy:
			update.Delta = &Update_ReplacedProxy{
				ReplacedProxy: &Update_ReplacedProxySettings{},
			}
		case *Update_AnsReplaceTargetPools:
			consumed := &Update_ReplacedTargetPools{
				Handle: delta.AnsReplaceTargetPools.Handle,
				Roles:  map[string]*Update_ReplacedTargetPools_TargetPools{},
			}

			for k, v := range delta.AnsReplaceTargetPools.Roles {
				consumed.Roles[k] = &Update_ReplacedTargetPools_TargetPools{
					Pools: v.Pools,
				}
			}

			update.Delta = &Update_ReplacedTargetPools_{
				ReplacedTargetPools: consumed,
			}
		case *Update_KpatchNodes:
			update.Delta = &Update_PatchedNodes_{
				PatchedNodes: &Update_PatchedNodes{},
			}
		case *Update_TfAddK8SNodes:
			consumed := &Update_AddedK8SNodes{
				NewNodePool: false,
				Nodepool:    "",
				Nodes:       []string{},
			}

			switch kind := delta.TfAddK8SNodes.Kind.(type) {
			case *Update_TerraformerAddK8SNodes_Existing_:
				consumed.NewNodePool = false
				consumed.Nodepool = kind.Existing.Nodepool
				for _, n := range kind.Existing.Nodes {
					consumed.Nodes = append(consumed.Nodes, n.Name)
				}
			case *Update_TerraformerAddK8SNodes_New_:
				consumed.NewNodePool = true
				consumed.Nodepool = kind.New.Nodepool.Name
				for _, n := range kind.New.Nodepool.Nodes {
					consumed.Nodes = append(consumed.Nodes, n.Name)
				}
			}

			update.Delta = &Update_AddedK8SNodes_{
				AddedK8SNodes: consumed,
			}
		case *Update_TfAddLoadBalancer:
			update.Delta = &Update_AddedLoadBalancer_{
				AddedLoadBalancer: &Update_AddedLoadBalancer{
					Handle: delta.TfAddLoadBalancer.Handle.ClusterInfo.Id(),
				},
			}
		case *Update_TfAddLoadBalancerNodes:
			consumed := &Update_AddedLoadBalancerNodes{
				Handle:      delta.TfAddLoadBalancerNodes.Handle,
				NewNodePool: false,
				NodePool:    "",
				Nodes:       []string{},
			}

			switch kind := delta.TfAddLoadBalancerNodes.Kind.(type) {
			case *Update_TerraformerAddLoadBalancerNodes_Existing_:
				consumed.NewNodePool = false
				consumed.NodePool = kind.Existing.Nodepool
				for _, n := range kind.Existing.Nodes {
					consumed.Nodes = append(consumed.Nodes, n.Name)
				}
			case *Update_TerraformerAddLoadBalancerNodes_New_:
				consumed.NewNodePool = true
				consumed.NodePool = kind.New.Nodepool.Name
				for _, n := range kind.New.Nodepool.Nodes {
					consumed.Nodes = append(consumed.Nodes, n.Name)
				}
			}

			update.Delta = &Update_AddedLoadBalancerNodes_{
				AddedLoadBalancerNodes: consumed,
			}
		case *Update_TfAddLoadBalancerRoles:
			roles := make([]string, 0, len(delta.TfAddLoadBalancerRoles.Roles))
			for _, r := range delta.TfAddLoadBalancerRoles.Roles {
				roles = append(roles, r.Name)
			}
			update.Delta = &Update_AddedLoadBalancerRoles_{
				AddedLoadBalancerRoles: &Update_AddedLoadBalancerRoles{
					Handle: delta.TfAddLoadBalancerRoles.Handle,
					Roles:  roles,
				},
			}
		case *Update_TfReplaceDns:
			update.Delta = &Update_ReplacedDns_{
				ReplacedDns: &Update_ReplacedDns{
					Handle:         delta.TfReplaceDns.Handle,
					OldApiEndpoint: delta.TfReplaceDns.OldApiEndpoint,
				},
			}
		default:
			// other messages are non-consumable, do nothing.
		}
	}

	return nil
}

// Returns mutable references to the underlying [Clusters] state stored
// within the [Task]. Any changes made to the returned [Clusters] will
// be reflected in the individual [Task] state.
//
// Each [Task] is spawned with a valid [Clusters] state, thus the function
// always returns fully valid data which was scheduled for the task.
//
// While this allows to directly mutate the returned [Clusters] it will not
// allow Clearing, i.e setting to nil. For this consider using [Task.ConsumeClearResult]
// or [Task.ConsumeUpdateResult]
func (te *Task) MutableClusters() (*Clusters, error) {
	state := Clusters{
		LoadBalancers: &LoadBalancers{},
	}

	switch task := te.Do.(type) {
	case *Task_Create:
		state.K8S = task.Create.K8S
		state.LoadBalancers.Clusters = task.Create.LoadBalancers
	case *Task_Delete:
		state.K8S = task.Delete.K8S
		state.LoadBalancers.Clusters = task.Delete.LoadBalancers
	case *Task_Update:
		state.K8S = task.Update.State.K8S
		state.LoadBalancers.Clusters = task.Update.State.LoadBalancers
	default:
		return nil, fmt.Errorf("unknown task %T", task)
	}

	return &state, nil
}

type InFlightUpdateState struct {
	r     *TaskResult
	state *TaskResult_UpdateState
}

func (s *InFlightUpdateState) Kubernetes(c *K8Scluster) *InFlightUpdateState {
	if c != nil {
		s.state.K8S = proto.Clone(c).(*K8Scluster)
	}
	return s
}

func (s *InFlightUpdateState) Loadbalancers(lbs ...*LBcluster) *InFlightUpdateState {
	if len(lbs) > 0 {
		s.state.LoadBalancers = new(LoadBalancers)
		for _, lb := range lbs {
			if lb != nil {
				lb := proto.Clone(lb).(*LBcluster)
				s.state.LoadBalancers.Clusters = append(s.state.LoadBalancers.Clusters, lb)
			}
		}
	}
	return s
}

// TODO: test me.
func (s *InFlightUpdateState) Commit() {
	switch prev := s.r.Result.(type) {
	case *TaskResult_Update:
		old := prev.Update
		new := s.state

		if new.K8S != nil {
			old.K8S = new.K8S
			new.K8S = nil
		}

		// update existing ones.
		for i := range old.LoadBalancers.Clusters {
			o := old.LoadBalancers.Clusters[i].ClusterInfo.Id()
			for j := range new.LoadBalancers.Clusters {
				if n := new.LoadBalancers.Clusters[j].ClusterInfo.Id(); n == o {
					old.LoadBalancers.Clusters[i] = new.LoadBalancers.Clusters[j]
					new.LoadBalancers.Clusters = slices.Delete(new.LoadBalancers.Clusters, j, j+1)
					break
				}
			}
		}

		// add new ones
		old.LoadBalancers.Clusters = append(old.LoadBalancers.Clusters, new.LoadBalancers.Clusters...)
		s.state = nil
	default:
		s.r.Result = &TaskResult_Update{
			Update: s.state,
		}
		s.state = nil
	}
}

func (r *TaskResult) Update() *InFlightUpdateState {
	return &InFlightUpdateState{
		r: r,
		state: &TaskResult_UpdateState{
			LoadBalancers: &LoadBalancers{},
		},
	}
}

type InFlightClearState struct {
	r     *TaskResult
	state *TaskResult_ClearState
}

func (s *InFlightClearState) Commit() {
	s.r.Result = &TaskResult_Clear{
		Clear: s.state,
	}
	s.state = nil
}

func (s *InFlightClearState) Kubernetes() *InFlightClearState {
	ok := true
	s.state.K8S = &ok
	return s
}

func (s *InFlightClearState) LoadBalancers(lbs ...string) *InFlightClearState {
	if len(lbs) > 0 {
		s.state.LoadBalancersIDs = []string{}
		for _, lb := range lbs {
			s.state.LoadBalancersIDs = append(s.state.LoadBalancersIDs, lb)
		}
	}
	return s
}

func (r *TaskResult) Clear() *InFlightClearState {
	return &InFlightClearState{
		r:     r,
		state: new(TaskResult_ClearState),
	}
}

func (r *TaskResult) IsNone() bool   { return r.GetNone() != nil }
func (r *TaskResult) IsUpdate() bool { return r.GetUpdate() != nil }
func (r *TaskResult) IsClear() bool  { return r.GetClear() != nil }
