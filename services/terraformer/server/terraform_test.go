package main

import (
	"fmt"
	"testing"

	"github.com/Berops/platform/proto/pb"
	"github.com/stretchr/testify/require"
)

var desiredState *pb.Project = &pb.Project{
	Name: "TestProjectName",
	Clusters: []*pb.K8Scluster{
		{
			ClusterInfo: &pb.ClusterInfo{
				Name:       "cluster1",
				PublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAAL2EmPjijvam+XCRMOThTzdDgqVc4+1Pu8mQH21CRAQGOsEyCfc8Qu6YN3wriEpgsARnmwWg3bqfWaP4qfAG6UfRra6QySmSYusVPDBfghxFQgSiZsBMFDy4EhsW89o+wHtN87Cvtys1Z2k+pcCTyIR4d6bK77eBjCFHvgCXNemHUtpHvcqI157rv/T4nB99aTWvRwGWwXX6l46iH7OD4m8UW/bZWBLSuWu9vSDFCrOUYDfl1s5KgjraXYIx2WW7CjqAxz5Zsk2zhiOiWk8igJWZJSP8iohq/TXrm2Zg9a8G4Bo73yH/XGQK3Y9a8HrDcaf7qx5lF1uRgkany7974k=",
				PrivateKey: "-----BEGIN RSA PRIVATE KEY-----\\nMIIEpQIBAAKCAQEA07lda1xyTiA3EDjd9HarcMC+7fTBdOPZ3j/hqZ4XGG+PhADQ\\nK4ByUXinHpe8KkFuDK32jBCkRtp9C5bcELUAqwECW8muTrcQmueIHWNKeRQpk02m\\nVxc8ok1KIQT+EJlxy9HnM7fB9UTDIzW+9Wajudcqm2pFjhfA9Lq2YzqMRg9W/g4V\\nF3o2tYFkU0VuFW/a3se0+FL14SdIf64ntpSe1HeuRboMHKAxIYz1eXrFUmbP9eCC\\nZdxv6AYkc++cKjtwB7+MN+qchB9PiKYt8XPpTOsiULRES7mWvN0/HkYF7RJAUUZe\\n8Y7xbAMYRszCF1etUhDJo7WYmGzAcnOabA17wQIDAQABAoIBAQCWY0lXFDBhDALw\\ns+P8XDqEeXRlI4AynhHICwooaBGxmrSPJ+C1taP9+LyEP3U5coZP3VWNwdheN52w\\nmLSx7U+PJ8VcG1KDuQbDcQg/D6L7g/XQGsy3edro0guAjsJEWin5g5w9IR5VMEWH\\nggqk1wYe4BX0lLTZLEgzzVf1/cVbkQjz7UvhTsqYvONFDEKdab59iEu+NK80xHcf\\npquM9OeW3R+qZThgFooIORKIHlIE66qeW9cYNIqztl3AfCzXdv90ciHhzRiyOVT0\\nOZR7tzSXEygKORKQsvlYO+Hj/kF/4Z7nryoVKFO2MVKLb8qzSuDtlu6Jx0V/ZHJO\\nwj+Z5QINAoGBAO97PvSZrz1/O/2BY80vz6EscXTIoRuTCBKDJHlY0tzcyjq2NmwI\\nd3l/P4LptDD5FfB6GkIpdGuh5B2GEFLH3hvYM1E6ySr4zxr+5LpiT/do5DiFnOV5\\nCzjFDR8wHK4lPHf4/FfsavvfUZQNfRciQ4pKpGgWEYmcsP5/7yFjI/ILAoGBAOJT\\n+xWQ/MVEjNwuhTmQ6uEn2RyTRkceQ08mnUvR7ZLObx0JW7fDQjeVQ1R+41W1xGI9\\nxEvRPLGAfdjZ2uFAKNaKLeMqJJS3W36RNbhcHRuGVJ+1phL19NoXs9MM127VRlr4\\n5ENpgFJG1F4G2OzZmzMUOgMJorh90hvMzMa07xTjAoGBAL496+8nvzxdPOzPwtaX\\napug0KhzUPi0vq7mGy2C0E+/3a7yXR1JRI/x9CQtP4W/+hvFA+MXR3LRcoO5onIA\\ncIMyJuIajwBiEzRg1Jbzzo6+dr4n9lGc7Ls2XowuDjqRPg4Yb23xU7Ou3gF9Dag5\\nAep0DVLaZSgqn7gtLWwac82tAoGAKTIyHL3UVK/il91b4JuRNUSEj1/7RcyrYcfc\\nj8V5YeRzcyyV5kADWIyxwbqK9LnuMheeGFLQolqKDaOx5JhCFrL2IUg1emBZphMW\\nXSVfIvhzhNKSlRbx55Sy5bKLsB/f+4UcP2z/r3o3A5ppd8swJb8DxDPHy58TVH4V\\ntAGRFxMCgYEAyHpY4meB1V4uqL8BFyzd4skT1uLGUWBv5GNEJFPGOuUmFfnvbH8O\\nmXGCrQFEw0qef7z2bBWZrAxHZOSrjyvn4Im8Vyr0AK6qQ1GZiOMnxb8kkQadGAye\\no6JnhZILhxE16ukh7fRfD4eI/Q6f1JT3eMc4mktackhtbUdOxYmUPuw=\\n-----END RSA PRIVATE KEY-----\\n",
				NodePools: []*pb.NodePool{
					{
						Name:       "NodePoolName1-Master",
						Region:     "Autralia",
						ServerType: "cpx11",
						Image:      "ubuntu-20.04",
						DiskSize:   20,
						Zone:       "example zone",
						Count:      1,
						Provider: &pb.Provider{
							Name:        "hetzner",
							Credentials: "xIAfsb7M5K6etYAfXYcg5iYyrFGNlCxcICo060HVEygjoF0usFpv5P9X7pk85Xe1",
						},
					},
					{
						Name:       "NodePoolName1-Worker",
						Region:     "Autralia",
						ServerType: "cpx11",
						Image:      "ubuntu-20.04",
						DiskSize:   20,
						Zone:       "example zone",
						Count:      2,
						Provider: &pb.Provider{
							Name:        "hetzner",
							Credentials: "xIAfsb7M5K6etYAfXYcg5iYyrFGNlCxcICo060HVEygjoF0usFpv5P9X7pk85Xe1",
						},
					},
					{
						Name:       "NodePoolName2-Master",
						Region:     "Autralia",
						ServerType: "e2-small",
						Image:      "ubuntu-os-cloud/ubuntu-2004-lts",
						DiskSize:   20,
						Zone:       "example zone",
						Count:      1,
						Provider: &pb.Provider{
							Name:        "gcp",
							Credentials: "/Users/samuelstolicny/Github/Berops/platform/keys/platform-296509-d6ddeb344e91.json",
						},
					},
					{
						Name:       "NodePoolName2-Worker",
						Region:     "Autralia",
						ServerType: "e2-small",
						Image:      "ubuntu-os-cloud/ubuntu-2004-lts",
						DiskSize:   20,
						Zone:       "example zone",
						Count:      2,
						Provider: &pb.Provider{
							Name:        "gcp",
							Credentials: "/Users/samuelstolicny/Github/Berops/platform/keys/platform-296509-d6ddeb344e91.json",
						},
					},
				},
			},
			Kubernetes: "19.0",
			Network:    "192.168.2.0/24",
			Kubeconfig: "ExampleKubeConfig",
		},
		{
			ClusterInfo: &pb.ClusterInfo{
				Name:       "cluster2",
				PublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAAN+WzDppE4DGWBtlGnF474oOAJVOAERN3THGfJ6cCqFdSBMuSyNz/BlLpw/GcM8Sw6iuI/aK0F2xkiiH1d2Sw7huRe+wgP30XGFVOqVqMnu0zS3OWV/gJ1bOaCiXa8NTltGqb4cCKec3QAyziVrcbbBasvhY4DDsbLpzkhseQIuLN72CRQE8vpwbUunRnbuo0GcMqKGDnpyInVwt2WXNnW3n16k59dR/Bpuhj5fjSA9sM0xAVyymPsRIpVl1mMDullRkYWXo5Qj8xlDTEd8kHZUcmd2aM3Br0bWzxnZbGRhA1HSWnPqEGymOC1iUJDKtFIx2mr30pce1r+RRYfZbA0=",
				PrivateKey: "-----BEGIN RSA PRIVATE KEY-----\\nMIIEpQIBAAKCAQEA07lda1xyTiA3EDjd9HarcMC+7fTBdOPZ3j/hqZ4XGG+PhADQ\\nK4ByUXinHpe8KkFuDK32jBCkRtp9C5bcELUAqwECW8muTrcQmueIHWNKeRQpk02m\\nVxc8ok1KIQT+EJlxy9HnM7fB9UTDIzW+9Wajudcqm2pFjhfA9Lq2YzqMRg9W/g4V\\nF3o2tYFkU0VuFW/a3se0+FL14SdIf64ntpSe1HeuRboMHKAxIYz1eXrFUmbP9eCC\\nZdxv6AYkc++cKjtwB7+MN+qchB9PiKYt8XPpTOsiULRES7mWvN0/HkYF7RJAUUZe\\n8Y7xbAMYRszCF1etUhDJo7WYmGzAcnOabA17wQIDAQABAoIBAQCWY0lXFDBhDALw\\ns+P8XDqEeXRlI4AynhHICwooaBGxmrSPJ+C1taP9+LyEP3U5coZP3VWNwdheN52w\\nmLSx7U+PJ8VcG1KDuQbDcQg/D6L7g/XQGsy3edro0guAjsJEWin5g5w9IR5VMEWH\\nggqk1wYe4BX0lLTZLEgzzVf1/cVbkQjz7UvhTsqYvONFDEKdab59iEu+NK80xHcf\\npquM9OeW3R+qZThgFooIORKIHlIE66qeW9cYNIqztl3AfCzXdv90ciHhzRiyOVT0\\nOZR7tzSXEygKORKQsvlYO+Hj/kF/4Z7nryoVKFO2MVKLb8qzSuDtlu6Jx0V/ZHJO\\nwj+Z5QINAoGBAO97PvSZrz1/O/2BY80vz6EscXTIoRuTCBKDJHlY0tzcyjq2NmwI\\nd3l/P4LptDD5FfB6GkIpdGuh5B2GEFLH3hvYM1E6ySr4zxr+5LpiT/do5DiFnOV5\\nCzjFDR8wHK4lPHf4/FfsavvfUZQNfRciQ4pKpGgWEYmcsP5/7yFjI/ILAoGBAOJT\\n+xWQ/MVEjNwuhTmQ6uEn2RyTRkceQ08mnUvR7ZLObx0JW7fDQjeVQ1R+41W1xGI9\\nxEvRPLGAfdjZ2uFAKNaKLeMqJJS3W36RNbhcHRuGVJ+1phL19NoXs9MM127VRlr4\\n5ENpgFJG1F4G2OzZmzMUOgMJorh90hvMzMa07xTjAoGBAL496+8nvzxdPOzPwtaX\\napug0KhzUPi0vq7mGy2C0E+/3a7yXR1JRI/x9CQtP4W/+hvFA+MXR3LRcoO5onIA\\ncIMyJuIajwBiEzRg1Jbzzo6+dr4n9lGc7Ls2XowuDjqRPg4Yb23xU7Ou3gF9Dag5\\nAep0DVLaZSgqn7gtLWwac82tAoGAKTIyHL3UVK/il91b4JuRNUSEj1/7RcyrYcfc\\nj8V5YeRzcyyV5kADWIyxwbqK9LnuMheeGFLQolqKDaOx5JhCFrL2IUg1emBZphMW\\nXSVfIvhzhNKSlRbx55Sy5bKLsB/f+4UcP2z/r3o3A5ppd8swJb8DxDPHy58TVH4V\\ntAGRFxMCgYEAyHpY4meB1V4uqL8BFyzd4skT1uLGUWBv5GNEJFPGOuUmFfnvbH8O\\nmXGCrQFEw0qef7z2bBWZrAxHZOSrjyvn4Im8Vyr0AK6qQ1GZiOMnxb8kkQadGAye\\no6JnhZILhxE16ukh7fRfD4eI/Q6f1JT3eMc4mktackhtbUdOxYmUPuw=\\n-----END RSA PRIVATE KEY-----\\n",
				NodePools: []*pb.NodePool{
					{
						Name:       "NodePoolName3-Master",
						Region:     "Autralia",
						ServerType: "cpx11",
						Image:      "ubuntu-20.04",
						DiskSize:   20,
						Zone:       "example zone",
						Count:      1,
						Provider: &pb.Provider{
							Name:        "hetzner",
							Credentials: "xIAfsb7M5K6etYAfXYcg5iYyrFGNlCxcICo060HVEygjoF0usFpv5P9X7pk85Xe1",
						},
					},
					{
						Name:       "NodePoolName3-Worker",
						Region:     "Autralia",
						ServerType: "cpx11",
						Image:      "ubuntu-20.04",
						DiskSize:   20,
						Zone:       "example zone",
						Count:      2,
						Provider: &pb.Provider{
							Name:        "hetzner",
							Credentials: "xIAfsb7M5K6etYAfXYcg5iYyrFGNlCxcICo060HVEygjoF0usFpv5P9X7pk85Xe1",
						},
					},
					{
						Name:       "NodePoolName4-Master",
						Region:     "Autralia",
						ServerType: "e2-small",
						Image:      "ubuntu-os-cloud/ubuntu-2004-lts",
						DiskSize:   20,
						Zone:       "example zone",
						Count:      1,
						Provider: &pb.Provider{
							Name:        "gcp",
							Credentials: "/Users/samuelstolicny/Github/Berops/platform/keys/platform-296509-d6ddeb344e91.json",
						},
					},
					{
						Name:       "NodePoolName4-Worker",
						Region:     "Autralia",
						ServerType: "e2-small",
						Image:      "ubuntu-os-cloud/ubuntu-2004-lts",
						DiskSize:   20,
						Zone:       "example zone",
						Count:      2,
						Provider: &pb.Provider{
							Name:        "gcp",
							Credentials: "/Users/samuelstolicny/Github/Berops/platform/keys/platform-296509-d6ddeb344e91.json",
						},
					},
				},
			},

			Kubernetes: "20.1",
			Network:    "192.168.2.0/24",
			Kubeconfig: "ExampleKubeConfig",
		},
	},
	LoadBalancerClusters: []*pb.LBcluster{
		{
			ClusterInfo: &pb.ClusterInfo{
				Name:       "cluster1-api-server",
				PublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAAN+WzDppE4DGWBtlGnF474oOAJVOAERN3THGfJ6cCqFdSBMuSyNz/BlLpw/GcM8Sw6iuI/aK0F2xkiiH1d2Sw7huRe+wgP30XGFVOqVqMnu0zS3OWV/gJ1bOaCiXa8NTltGqb4cCKec3QAyziVrcbbBasvhY4DDsbLpzkhseQIuLN72CRQE8vpwbUunRnbuo0GcMqKGDnpyInVwt2WXNnW3n16k59dR/Bpuhj5fjSA9sM0xAVyymPsRIpVl1mMDullRkYWXo5Qj8xlDTEd8kHZUcmd2aM3Br0bWzxnZbGRhA1HSWnPqEGymOC1iUJDKtFIx2mr30pce1r+RRYfZbA0=",
				PrivateKey: "-----BEGIN RSA PRIVATE KEY-----\\nMIIEpQIBAAKCAQEA07lda1xyTiA3EDjd9HarcMC+7fTBdOPZ3j/hqZ4XGG+PhADQ\\nK4ByUXinHpe8KkFuDK32jBCkRtp9C5bcELUAqwECW8muTrcQmueIHWNKeRQpk02m\\nVxc8ok1KIQT+EJlxy9HnM7fB9UTDIzW+9Wajudcqm2pFjhfA9Lq2YzqMRg9W/g4V\\nF3o2tYFkU0VuFW/a3se0+FL14SdIf64ntpSe1HeuRboMHKAxIYz1eXrFUmbP9eCC\\nZdxv6AYkc++cKjtwB7+MN+qchB9PiKYt8XPpTOsiULRES7mWvN0/HkYF7RJAUUZe\\n8Y7xbAMYRszCF1etUhDJo7WYmGzAcnOabA17wQIDAQABAoIBAQCWY0lXFDBhDALw\\ns+P8XDqEeXRlI4AynhHICwooaBGxmrSPJ+C1taP9+LyEP3U5coZP3VWNwdheN52w\\nmLSx7U+PJ8VcG1KDuQbDcQg/D6L7g/XQGsy3edro0guAjsJEWin5g5w9IR5VMEWH\\nggqk1wYe4BX0lLTZLEgzzVf1/cVbkQjz7UvhTsqYvONFDEKdab59iEu+NK80xHcf\\npquM9OeW3R+qZThgFooIORKIHlIE66qeW9cYNIqztl3AfCzXdv90ciHhzRiyOVT0\\nOZR7tzSXEygKORKQsvlYO+Hj/kF/4Z7nryoVKFO2MVKLb8qzSuDtlu6Jx0V/ZHJO\\nwj+Z5QINAoGBAO97PvSZrz1/O/2BY80vz6EscXTIoRuTCBKDJHlY0tzcyjq2NmwI\\nd3l/P4LptDD5FfB6GkIpdGuh5B2GEFLH3hvYM1E6ySr4zxr+5LpiT/do5DiFnOV5\\nCzjFDR8wHK4lPHf4/FfsavvfUZQNfRciQ4pKpGgWEYmcsP5/7yFjI/ILAoGBAOJT\\n+xWQ/MVEjNwuhTmQ6uEn2RyTRkceQ08mnUvR7ZLObx0JW7fDQjeVQ1R+41W1xGI9\\nxEvRPLGAfdjZ2uFAKNaKLeMqJJS3W36RNbhcHRuGVJ+1phL19NoXs9MM127VRlr4\\n5ENpgFJG1F4G2OzZmzMUOgMJorh90hvMzMa07xTjAoGBAL496+8nvzxdPOzPwtaX\\napug0KhzUPi0vq7mGy2C0E+/3a7yXR1JRI/x9CQtP4W/+hvFA+MXR3LRcoO5onIA\\ncIMyJuIajwBiEzRg1Jbzzo6+dr4n9lGc7Ls2XowuDjqRPg4Yb23xU7Ou3gF9Dag5\\nAep0DVLaZSgqn7gtLWwac82tAoGAKTIyHL3UVK/il91b4JuRNUSEj1/7RcyrYcfc\\nj8V5YeRzcyyV5kADWIyxwbqK9LnuMheeGFLQolqKDaOx5JhCFrL2IUg1emBZphMW\\nXSVfIvhzhNKSlRbx55Sy5bKLsB/f+4UcP2z/r3o3A5ppd8swJb8DxDPHy58TVH4V\\ntAGRFxMCgYEAyHpY4meB1V4uqL8BFyzd4skT1uLGUWBv5GNEJFPGOuUmFfnvbH8O\\nmXGCrQFEw0qef7z2bBWZrAxHZOSrjyvn4Im8Vyr0AK6qQ1GZiOMnxb8kkQadGAye\\no6JnhZILhxE16ukh7fRfD4eI/Q6f1JT3eMc4mktackhtbUdOxYmUPuw=\\n-----END RSA PRIVATE KEY-----\\n",
				NodePools: []*pb.NodePool{
					{
						Name:       "NodePoolName-LB",
						Region:     "Autralia",
						ServerType: "e2-small",
						Image:      "ubuntu-os-cloud/ubuntu-2004-lts",
						DiskSize:   20,
						Zone:       "example zone",
						Count:      2,
						Provider: &pb.Provider{
							Name:        "gcp",
							Credentials: "/Users/samuelstolicny/Github/Berops/platform/keys/platform-296509-d6ddeb344e91.json",
						},
					},
				},
			},
			Roles: []*pb.Role{
				{
					Name:       "api-server-lb",
					Port:       6443,
					TargetPort: 6443,
					Target:     pb.Target_k8sControlPlane,
				},
			},
			Dns: &pb.DNS{
				Hostname: "www.test.io",
				Providers: []string{
					"gcp",
				},
			},
		},
	},
}

var jsonData = "{\"compute\":{\"test-cluster-compute1\":\"0.0.0.65\",\n\"test-cluster-compute2\":\"0.0.0.512\"},\n\"control\":{\"test-cluster-control1\":\"0.0.0.72\",\n\"test-cluster-control2\":\"0.0.0.65\"}}"

var testState *pb.Project = &pb.Project{
	Name: "TestProjectName",
	Clusters: []*pb.K8Scluster{
		{
			ClusterInfo: &pb.ClusterInfo{
				Name:       "cluster1",
				PublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAAL2EmPjijvam+XCRMOThTzdDgqVc4+1Pu8mQH21CRAQGOsEyCfc8Qu6YN3wriEpgsARnmwWg3bqfWaP4qfAG6UfRra6QySmSYusVPDBfghxFQgSiZsBMFDy4EhsW89o+wHtN87Cvtys1Z2k+pcCTyIR4d6bK77eBjCFHvgCXNemHUtpHvcqI157rv/T4nB99aTWvRwGWwXX6l46iH7OD4m8UW/bZWBLSuWu9vSDFCrOUYDfl1s5KgjraXYIx2WW7CjqAxz5Zsk2zhiOiWk8igJWZJSP8iohq/TXrm2Zg9a8G4Bo73yH/XGQK3Y9a8HrDcaf7qx5lF1uRgkany7974k=",
				PrivateKey: "-----BEGIN RSA PRIVATE KEY-----\\nMIIEpQIBAAKCAQEA07lda1xyTiA3EDjd9HarcMC+7fTBdOPZ3j/hqZ4XGG+PhADQ\\nK4ByUXinHpe8KkFuDK32jBCkRtp9C5bcELUAqwECW8muTrcQmueIHWNKeRQpk02m\\nVxc8ok1KIQT+EJlxy9HnM7fB9UTDIzW+9Wajudcqm2pFjhfA9Lq2YzqMRg9W/g4V\\nF3o2tYFkU0VuFW/a3se0+FL14SdIf64ntpSe1HeuRboMHKAxIYz1eXrFUmbP9eCC\\nZdxv6AYkc++cKjtwB7+MN+qchB9PiKYt8XPpTOsiULRES7mWvN0/HkYF7RJAUUZe\\n8Y7xbAMYRszCF1etUhDJo7WYmGzAcnOabA17wQIDAQABAoIBAQCWY0lXFDBhDALw\\ns+P8XDqEeXRlI4AynhHICwooaBGxmrSPJ+C1taP9+LyEP3U5coZP3VWNwdheN52w\\nmLSx7U+PJ8VcG1KDuQbDcQg/D6L7g/XQGsy3edro0guAjsJEWin5g5w9IR5VMEWH\\nggqk1wYe4BX0lLTZLEgzzVf1/cVbkQjz7UvhTsqYvONFDEKdab59iEu+NK80xHcf\\npquM9OeW3R+qZThgFooIORKIHlIE66qeW9cYNIqztl3AfCzXdv90ciHhzRiyOVT0\\nOZR7tzSXEygKORKQsvlYO+Hj/kF/4Z7nryoVKFO2MVKLb8qzSuDtlu6Jx0V/ZHJO\\nwj+Z5QINAoGBAO97PvSZrz1/O/2BY80vz6EscXTIoRuTCBKDJHlY0tzcyjq2NmwI\\nd3l/P4LptDD5FfB6GkIpdGuh5B2GEFLH3hvYM1E6ySr4zxr+5LpiT/do5DiFnOV5\\nCzjFDR8wHK4lPHf4/FfsavvfUZQNfRciQ4pKpGgWEYmcsP5/7yFjI/ILAoGBAOJT\\n+xWQ/MVEjNwuhTmQ6uEn2RyTRkceQ08mnUvR7ZLObx0JW7fDQjeVQ1R+41W1xGI9\\nxEvRPLGAfdjZ2uFAKNaKLeMqJJS3W36RNbhcHRuGVJ+1phL19NoXs9MM127VRlr4\\n5ENpgFJG1F4G2OzZmzMUOgMJorh90hvMzMa07xTjAoGBAL496+8nvzxdPOzPwtaX\\napug0KhzUPi0vq7mGy2C0E+/3a7yXR1JRI/x9CQtP4W/+hvFA+MXR3LRcoO5onIA\\ncIMyJuIajwBiEzRg1Jbzzo6+dr4n9lGc7Ls2XowuDjqRPg4Yb23xU7Ou3gF9Dag5\\nAep0DVLaZSgqn7gtLWwac82tAoGAKTIyHL3UVK/il91b4JuRNUSEj1/7RcyrYcfc\\nj8V5YeRzcyyV5kADWIyxwbqK9LnuMheeGFLQolqKDaOx5JhCFrL2IUg1emBZphMW\\nXSVfIvhzhNKSlRbx55Sy5bKLsB/f+4UcP2z/r3o3A5ppd8swJb8DxDPHy58TVH4V\\ntAGRFxMCgYEAyHpY4meB1V4uqL8BFyzd4skT1uLGUWBv5GNEJFPGOuUmFfnvbH8O\\nmXGCrQFEw0qef7z2bBWZrAxHZOSrjyvn4Im8Vyr0AK6qQ1GZiOMnxb8kkQadGAye\\no6JnhZILhxE16ukh7fRfD4eI/Q6f1JT3eMc4mktackhtbUdOxYmUPuw=\\n-----END RSA PRIVATE KEY-----\\n",
				NodePools: []*pb.NodePool{
					{
						Name:       "NodePoolName1-Master",
						Region:     "Autralia",
						ServerType: "cpx11",
						Image:      "ubuntu-20.04",
						DiskSize:   20,
						Zone:       "example zone",
						Count:      3,
						Provider: &pb.Provider{
							Name:        "hetzner",
							Credentials: "xIAfsb7M5K6etYAfXYcg5iYyrFGNlCxcICo060HVEygjoF0usFpv5P9X7pk85Xe1",
						},
					},
					{
						Name:       "NodePoolName1-Worker",
						Region:     "Autralia",
						ServerType: "cpx11",
						Image:      "ubuntu-20.04",
						DiskSize:   20,
						Zone:       "example zone",
						Count:      3,
						Provider: &pb.Provider{
							Name:        "hetzner",
							Credentials: "xIAfsb7M5K6etYAfXYcg5iYyrFGNlCxcICo060HVEygjoF0usFpv5P9X7pk85Xe1",
						},
					},
				},
			},
			Kubernetes: "20.1",
			Network:    "192.168.2.0/24",
			Kubeconfig: "ExampleKubeConfig",
		},
	},
}

func TestBuildInfrastructure(t *testing.T) {
	err := buildInfrastructure(testState, testState)
	require.NoError(t, err)
}

func TestOutputTerraform(t *testing.T) {
	out, err := outputTerraform(outputPath, testState.Clusters[0].ClusterInfo.NodePools[0])
	t.Log(out)
	require.NoError(t, err)
}

func TestReadOutput(t *testing.T) {
	out, err := readOutput(jsonData)

	if err == nil {
		t.Log(out.IPs)
	}
	require.NoError(t, err)
}

func TestFillNodes(t *testing.T) {
	out, err := readOutput(jsonData)
	if err == nil {
		var m = &pb.NodePool{}
		fillNodes(&out, m, desiredState.Clusters[0].ClusterInfo.NodePools[0].Nodes)
		fmt.Println(m)
	}
	require.NoError(t, err)
}

func TestBuildLBNodepools(t *testing.T) {
	err := buildNodePools(desiredState.LoadBalancerClusters[0].GetClusterInfo(), "terraform", "-lb.tpl", "-lb.tf")
	require.NoError(t, err)
}

func TestBuildNodepools(t *testing.T) {
	err := buildNodePools(desiredState.Clusters[0].ClusterInfo, "terraform", ".tpl", ".tf")
	require.NoError(t, err)
}
