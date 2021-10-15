package terraformer

import (
	"testing"

	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/utils"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
)

var desiredState *pb.Project = &pb.Project{
	Name: "TestProjectName",
	Clusters: []*pb.Cluster{
		{
			Name:       "cluster1",
			Kubernetes: "20.1",
			Network:    "192.168.2.0/24",
			Kubeconfig: "ExampleKubeConfig",
			PublicKey:  "-----BEGIN RSA PUBLIC KEY-----\\nMIIBCgKCAQEA07lda1xyTiA3EDjd9HarcMC+7fTBdOPZ3j/hqZ4XGG+PhADQK4By\\nUXinHpe8KkFuDK32jBCkRtp9C5bcELUAqwECW8muTrcQmueIHWNKeRQpk02mVxc8\\nok1KIQT+EJlxy9HnM7fB9UTDIzW+9Wajudcqm2pFjhfA9Lq2YzqMRg9W/g4VF3o2\\ntYFkU0VuFW/a3se0+FL14SdIf64ntpSe1HeuRboMHKAxIYz1eXrFUmbP9eCCZdxv\\n6AYkc++cKjtwB7+MN+qchB9PiKYt8XPpTOsiULRES7mWvN0/HkYF7RJAUUZe8Y7x\\nbAMYRszCF1etUhDJo7WYmGzAcnOabA17wQIDAQAB\\n-----END RSA PUBLIC KEY-----\\n",
			PrivateKey: "-----BEGIN RSA PRIVATE KEY-----\\nMIIEpQIBAAKCAQEA07lda1xyTiA3EDjd9HarcMC+7fTBdOPZ3j/hqZ4XGG+PhADQ\\nK4ByUXinHpe8KkFuDK32jBCkRtp9C5bcELUAqwECW8muTrcQmueIHWNKeRQpk02m\\nVxc8ok1KIQT+EJlxy9HnM7fB9UTDIzW+9Wajudcqm2pFjhfA9Lq2YzqMRg9W/g4V\\nF3o2tYFkU0VuFW/a3se0+FL14SdIf64ntpSe1HeuRboMHKAxIYz1eXrFUmbP9eCC\\nZdxv6AYkc++cKjtwB7+MN+qchB9PiKYt8XPpTOsiULRES7mWvN0/HkYF7RJAUUZe\\n8Y7xbAMYRszCF1etUhDJo7WYmGzAcnOabA17wQIDAQABAoIBAQCWY0lXFDBhDALw\\ns+P8XDqEeXRlI4AynhHICwooaBGxmrSPJ+C1taP9+LyEP3U5coZP3VWNwdheN52w\\nmLSx7U+PJ8VcG1KDuQbDcQg/D6L7g/XQGsy3edro0guAjsJEWin5g5w9IR5VMEWH\\nggqk1wYe4BX0lLTZLEgzzVf1/cVbkQjz7UvhTsqYvONFDEKdab59iEu+NK80xHcf\\npquM9OeW3R+qZThgFooIORKIHlIE66qeW9cYNIqztl3AfCzXdv90ciHhzRiyOVT0\\nOZR7tzSXEygKORKQsvlYO+Hj/kF/4Z7nryoVKFO2MVKLb8qzSuDtlu6Jx0V/ZHJO\\nwj+Z5QINAoGBAO97PvSZrz1/O/2BY80vz6EscXTIoRuTCBKDJHlY0tzcyjq2NmwI\\nd3l/P4LptDD5FfB6GkIpdGuh5B2GEFLH3hvYM1E6ySr4zxr+5LpiT/do5DiFnOV5\\nCzjFDR8wHK4lPHf4/FfsavvfUZQNfRciQ4pKpGgWEYmcsP5/7yFjI/ILAoGBAOJT\\n+xWQ/MVEjNwuhTmQ6uEn2RyTRkceQ08mnUvR7ZLObx0JW7fDQjeVQ1R+41W1xGI9\\nxEvRPLGAfdjZ2uFAKNaKLeMqJJS3W36RNbhcHRuGVJ+1phL19NoXs9MM127VRlr4\\n5ENpgFJG1F4G2OzZmzMUOgMJorh90hvMzMa07xTjAoGBAL496+8nvzxdPOzPwtaX\\napug0KhzUPi0vq7mGy2C0E+/3a7yXR1JRI/x9CQtP4W/+hvFA+MXR3LRcoO5onIA\\ncIMyJuIajwBiEzRg1Jbzzo6+dr4n9lGc7Ls2XowuDjqRPg4Yb23xU7Ou3gF9Dag5\\nAep0DVLaZSgqn7gtLWwac82tAoGAKTIyHL3UVK/il91b4JuRNUSEj1/7RcyrYcfc\\nj8V5YeRzcyyV5kADWIyxwbqK9LnuMheeGFLQolqKDaOx5JhCFrL2IUg1emBZphMW\\nXSVfIvhzhNKSlRbx55Sy5bKLsB/f+4UcP2z/r3o3A5ppd8swJb8DxDPHy58TVH4V\\ntAGRFxMCgYEAyHpY4meB1V4uqL8BFyzd4skT1uLGUWBv5GNEJFPGOuUmFfnvbH8O\\nmXGCrQFEw0qef7z2bBWZrAxHZOSrjyvn4Im8Vyr0AK6qQ1GZiOMnxb8kkQadGAye\\no6JnhZILhxE16ukh7fRfD4eI/Q6f1JT3eMc4mktackhtbUdOxYmUPuw=\\n-----END RSA PRIVATE KEY-----\\n",
			NodePools: []*pb.NodePool{
				{
					Name:   "NodePoolName1",
					Region: "Autralia",
					Master: &pb.Node{
						Count:      1,
						ServerType: "cpx11",
						Image:      "ubuntu-20.04",
						DiskSize:   20,
						Zone:       "example zone",
						Location:   "example loca",
						Datacenter: "example datacenter",
					},
					Worker: &pb.Node{
						Count:      2,
						ServerType: "cpx11",
						Image:      "ubuntu-20.04",
						DiskSize:   20,
						Zone:       "example zone",
						Location:   "example loca",
						Datacenter: "example datacenter",
					},
					Provider: &pb.Provider{
						Name:        "hetzner",
						Credentials: "xIAfsb7M5K6etYAfXYcg5iYyrFGNlCxcICo060HVEygjoF0usFpv5P9X7pk85Xe1",
					},
				},
				{
					Name:   "NodePoolName2",
					Region: "Autralia",
					Master: &pb.Node{
						Count:      1,
						ServerType: "e2-small",
						Image:      "ubuntu-os-cloud/ubuntu-2004-lts",
						DiskSize:   20,
					},
					Worker: &pb.Node{
						Count:      2,
						ServerType: "e2-small",
						Image:      "ubuntu-os-cloud/ubuntu-2004-lts",
						DiskSize:   20,
					},
					Provider: &pb.Provider{
						Name:        "gcp",
						Credentials: "/Users/samuelstolicny/Github/Berops/platform/keys/platform-296509-d6ddeb344e91.json",
					},
				},
			},
		},
		{
			Name:       "cluster2",
			Kubernetes: "20.1",
			Network:    "192.168.2.0/24",
			Kubeconfig: "ExampleKubeConfig",
			PublicKey:  "-----BEGIN RSA PUBLIC KEY-----\\nMIIBCgKCAQEA07lda1xyTiA3EDjd9HarcMC+7fTBdOPZ3j/hqZ4XGG+PhADQK4By\\nUXinHpe8KkFuDK32jBCkRtp9C5bcELUAqwECW8muTrcQmueIHWNKeRQpk02mVxc8\\nok1KIQT+EJlxy9HnM7fB9UTDIzW+9Wajudcqm2pFjhfA9Lq2YzqMRg9W/g4VF3o2\\ntYFkU0VuFW/a3se0+FL14SdIf64ntpSe1HeuRboMHKAxIYz1eXrFUmbP9eCCZdxv\\n6AYkc++cKjtwB7+MN+qchB9PiKYt8XPpTOsiULRES7mWvN0/HkYF7RJAUUZe8Y7x\\nbAMYRszCF1etUhDJo7WYmGzAcnOabA17wQIDAQAB\\n-----END RSA PUBLIC KEY-----\\n",
			PrivateKey: "-----BEGIN RSA PRIVATE KEY-----\\nMIIEpQIBAAKCAQEA07lda1xyTiA3EDjd9HarcMC+7fTBdOPZ3j/hqZ4XGG+PhADQ\\nK4ByUXinHpe8KkFuDK32jBCkRtp9C5bcELUAqwECW8muTrcQmueIHWNKeRQpk02m\\nVxc8ok1KIQT+EJlxy9HnM7fB9UTDIzW+9Wajudcqm2pFjhfA9Lq2YzqMRg9W/g4V\\nF3o2tYFkU0VuFW/a3se0+FL14SdIf64ntpSe1HeuRboMHKAxIYz1eXrFUmbP9eCC\\nZdxv6AYkc++cKjtwB7+MN+qchB9PiKYt8XPpTOsiULRES7mWvN0/HkYF7RJAUUZe\\n8Y7xbAMYRszCF1etUhDJo7WYmGzAcnOabA17wQIDAQABAoIBAQCWY0lXFDBhDALw\\ns+P8XDqEeXRlI4AynhHICwooaBGxmrSPJ+C1taP9+LyEP3U5coZP3VWNwdheN52w\\nmLSx7U+PJ8VcG1KDuQbDcQg/D6L7g/XQGsy3edro0guAjsJEWin5g5w9IR5VMEWH\\nggqk1wYe4BX0lLTZLEgzzVf1/cVbkQjz7UvhTsqYvONFDEKdab59iEu+NK80xHcf\\npquM9OeW3R+qZThgFooIORKIHlIE66qeW9cYNIqztl3AfCzXdv90ciHhzRiyOVT0\\nOZR7tzSXEygKORKQsvlYO+Hj/kF/4Z7nryoVKFO2MVKLb8qzSuDtlu6Jx0V/ZHJO\\nwj+Z5QINAoGBAO97PvSZrz1/O/2BY80vz6EscXTIoRuTCBKDJHlY0tzcyjq2NmwI\\nd3l/P4LptDD5FfB6GkIpdGuh5B2GEFLH3hvYM1E6ySr4zxr+5LpiT/do5DiFnOV5\\nCzjFDR8wHK4lPHf4/FfsavvfUZQNfRciQ4pKpGgWEYmcsP5/7yFjI/ILAoGBAOJT\\n+xWQ/MVEjNwuhTmQ6uEn2RyTRkceQ08mnUvR7ZLObx0JW7fDQjeVQ1R+41W1xGI9\\nxEvRPLGAfdjZ2uFAKNaKLeMqJJS3W36RNbhcHRuGVJ+1phL19NoXs9MM127VRlr4\\n5ENpgFJG1F4G2OzZmzMUOgMJorh90hvMzMa07xTjAoGBAL496+8nvzxdPOzPwtaX\\napug0KhzUPi0vq7mGy2C0E+/3a7yXR1JRI/x9CQtP4W/+hvFA+MXR3LRcoO5onIA\\ncIMyJuIajwBiEzRg1Jbzzo6+dr4n9lGc7Ls2XowuDjqRPg4Yb23xU7Ou3gF9Dag5\\nAep0DVLaZSgqn7gtLWwac82tAoGAKTIyHL3UVK/il91b4JuRNUSEj1/7RcyrYcfc\\nj8V5YeRzcyyV5kADWIyxwbqK9LnuMheeGFLQolqKDaOx5JhCFrL2IUg1emBZphMW\\nXSVfIvhzhNKSlRbx55Sy5bKLsB/f+4UcP2z/r3o3A5ppd8swJb8DxDPHy58TVH4V\\ntAGRFxMCgYEAyHpY4meB1V4uqL8BFyzd4skT1uLGUWBv5GNEJFPGOuUmFfnvbH8O\\nmXGCrQFEw0qef7z2bBWZrAxHZOSrjyvn4Im8Vyr0AK6qQ1GZiOMnxb8kkQadGAye\\no6JnhZILhxE16ukh7fRfD4eI/Q6f1JT3eMc4mktackhtbUdOxYmUPuw=\\n-----END RSA PRIVATE KEY-----\\n",
			NodePools: []*pb.NodePool{
				{
					Name:   "NodePoolName3",
					Region: "Autralia",
					Master: &pb.Node{
						Count:      1,
						ServerType: "cpx11",
						Image:      "ubuntu-20.04",
						DiskSize:   20,
						Zone:       "example zone",
						Location:   "example loca",
						Datacenter: "example datacenter",
					},
					Worker: &pb.Node{
						Count:      2,
						ServerType: "cpx11",
						Image:      "ubuntu-20.04",
						DiskSize:   20,
						Zone:       "example zone",
						Location:   "example loca",
						Datacenter: "example datacenter",
					},
					Provider: &pb.Provider{
						Name:        "hetzner",
						Credentials: "xIAfsb7M5K6etYAfXYcg5iYyrFGNlCxcICo060HVEygjoF0usFpv5P9X7pk85Xe1",
					},
				},
				{
					Name:   "NodePoolName4",
					Region: "Autralia",
					Master: &pb.Node{
						Count:      1,
						ServerType: "e2-small",
						Image:      "ubuntu-os-cloud/ubuntu-2004-lts",
						DiskSize:   20,
					},
					Worker: &pb.Node{
						Count:      2,
						ServerType: "e2-small",
						Image:      "ubuntu-os-cloud/ubuntu-2004-lts",
						DiskSize:   20,
					},
					Provider: &pb.Provider{
						Name:        "gcp",
						Credentials: "/Users/samuelstolicny/Github/Berops/platform/keys/platform-296509-d6ddeb344e91.json",
					},
				},
			},
		},
	},
}

func TestBuildInfrastructure(t *testing.T) {
	// Create connection to Terraformer
	cc, err := utils.GrpcDialWithInsecure("terraformer", utils.TerraformerURL)
	if err != nil {
		log.Fatal().Err(err)
	}
	defer func() {
		err := cc.Close()
		require.NoError(t, err)
	}()
	// Creating the client
	c := pb.NewTerraformerServiceClient(cc)

	res, err := BuildInfrastructure(c, &pb.BuildInfrastructureRequest{
		Config: &pb.Config{
			Id:           "12345",
			Name:         "Test config for Terraformer",
			Manifest:     "ManifestStringExample",
			DesiredState: desiredState,
		},
	})
	require.NoError(t, err)
	t.Log("Terraformer response: ", res)

	// Print just public ip addresses
	for _, cluster := range res.GetConfig().GetCurrentState().GetClusters() {
		t.Log(cluster.GetName())
		for k, ip := range cluster.GetNodeInfos() {
			t.Log(k, ip.GetPublic())
		}
	}
}
