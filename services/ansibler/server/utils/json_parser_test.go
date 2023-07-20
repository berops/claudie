package utils

import (
	"strings"
	"testing"
)

func TestCollectErrors(t *testing.T) {
	tests := []struct {
		Name    string
		Input   string
		WantErr bool
		ErrMsg  string
	}{
		{
			Name: "test-set-0",
			Input: `
{
    "custom_stats": {},
    "global_custom_stats": {},
    "plays": [
        {
            "play": {
                "duration": {
                    "end": "2023-07-10T15:41:26.442875Z",
                    "start": "2023-07-10T15:36:11.262893Z"
                },
                "id": "56ebfbb3-6424-e6c6-fa10-000000000003",
                "name": "all",
                "path": "./github.com/Berops/claudie/services/ansibler/server/ansible-playbooks/longhorn-req.yml:2"
            },
            "tasks": [
                {
                    "hosts": {
                        "hetzner-mh49kks-1": {
                            "_ansible_no_log": null,
                            "action": "wait_for_connection",
                            "changed": false,
                            "elapsed": 308,
                            "failed": true,
                            "msg": "timed out waiting for ping module test: Failed to connect to the host via ssh: ssh: connect to host 23.88.53.227 port 22: Operation timed out"
                        },
                        "hetzner-zi5vl5z-1": {
                            "_ansible_no_log": null,
                            "action": "wait_for_connection",
                            "changed": false,
                            "elapsed": 2
                        }
                    },
                    "task": {
                        "duration": {
                            "end": "2023-07-10T15:41:20.232551Z",
                            "start": "2023-07-10T15:36:11.268155Z"
                        },
                        "id": "56ebfbb3-6424-e6c6-fa10-000000000005",
                        "name": "Wait 300 seconds for target connection to become reachable/usable",
                        "path": "./github.com/Berops/claudie/services/ansibler/server/ansible-playbooks/longhorn-req.yml:8"
                    }
                },
                {
                    "hosts": {
                        "hetzner-zi5vl5z-1": {
                            "_ansible_no_log": null,
                            "action": "ansible.builtin.apt",
                            "ansible_facts": {
                                "discovered_interpreter_python": "/usr/bin/python3"
                            },
                            "cache_update_time": 1689003683,
                            "cache_updated": true,
                            "changed": false,
                            "invocation": {
                                "module_args": {
                                    "allow_change_held_packages": false,
                                    "allow_downgrade": false,
                                    "allow_unauthenticated": false,
                                    "autoclean": false,
                                    "autoremove": false,
                                    "cache_valid_time": 0,
                                    "clean": false,
                                    "deb": null,
                                    "default_release": null,
                                    "dpkg_options": "force-confdef,force-confold",
                                    "fail_on_autoremove": false,
                                    "force": false,
                                    "force_apt_get": false,
                                    "install_recommends": null,
                                    "lock_timeout": 60,
                                    "name": "open-iscsi",
                                    "only_upgrade": false,
                                    "package": [
                                        "open-iscsi"
                                    ],
                                    "policy_rc_d": null,
                                    "purge": false,
                                    "state": "present",
                                    "update_cache": true,
                                    "update_cache_retries": 5,
                                    "update_cache_retry_max_delay": 12,
                                    "upgrade": null
                                }
                            }
                        }
                    },
                    "task": {
                        "duration": {
                            "end": "2023-07-10T15:41:23.993898Z",
                            "start": "2023-07-10T15:41:20.252345Z"
                        },
                        "id": "56ebfbb3-6424-e6c6-fa10-000000000007",
                        "name": "install open-iscsi",
                        "path": "./github.com/Berops/claudie/services/ansibler/server/ansible-playbooks/longhorn-req.yml:14"
                    }
                },
                {
                    "hosts": {
                        "hetzner-zi5vl5z-1": {
                            "_ansible_no_log": null,
                            "action": "ansible.builtin.apt",
                            "cache_update_time": 1689003685,
                            "cache_updated": true,
                            "changed": false,
                            "invocation": {
                                "module_args": {
                                    "allow_change_held_packages": false,
                                    "allow_downgrade": false,
                                    "allow_unauthenticated": false,
                                    "autoclean": false,
                                    "autoremove": false,
                                    "cache_valid_time": 0,
                                    "clean": false,
                                    "deb": null,
                                    "default_release": null,
                                    "dpkg_options": "force-confdef,force-confold",
                                    "fail_on_autoremove": false,
                                    "force": false,
                                    "force_apt_get": false,
                                    "install_recommends": null,
                                    "lock_timeout": 60,
                                    "name": "nfs-common",
                                    "only_upgrade": false,
                                    "package": [
                                        "nfs-common"
                                    ],
                                    "policy_rc_d": null,
                                    "purge": false,
                                    "state": "present",
                                    "update_cache": true,
                                    "update_cache_retries": 5,
                                    "update_cache_retry_max_delay": 12,
                                    "upgrade": null
                                }
                            }
                        }
                    },
                    "task": {
                        "duration": {
                            "end": "2023-07-10T15:41:26.442875Z",
                            "start": "2023-07-10T15:41:23.999325Z"
                        },
                        "id": "56ebfbb3-6424-e6c6-fa10-000000000008",
                        "name": "install nfs-common",
                        "path": "./github.com/Berops/claudie/services/ansibler/server/ansible-playbooks/longhorn-req.yml:20"
                    }
                }
            ]
        }
    ],
    "stats": {
        "hetzner-mh49kks-1": {
            "changed": 0,
            "failures": 1,
            "ignored": 0,
            "ok": 0,
            "rescued": 0,
            "skipped": 0,
            "unreachable": 0
        },
        "hetzner-zi5vl5z-1": {
            "changed": 0,
            "failures": 0,
            "ignored": 0,
            "ok": 3,
            "rescued": 0,
            "skipped": 0,
            "unreachable": 0
        }
    }
}`,
			WantErr: true,
			ErrMsg:  "hetzner-mh49kks-1 failed inside task Wait 300 seconds for target connection to become reachable/usable due to: timed out waiting for ping module test: Failed to connect to the host via ssh: ssh: connect to host 23.88.53.227 port 22: Operation timed out",
		},
		{
			Name: "test-set-1",
			Input: `
		{
		   "custom_stats": {},
		   "global_custom_stats": {},
		   "plays": [
		       {
		           "play": {
		               "duration": {
		                   "end": "2023-07-10T15:41:26.442875Z",
		                   "start": "2023-07-10T15:36:11.262893Z"
		               },
		               "id": "56ebfbb3-6424-e6c6-fa10-000000000003",
		               "name": "all",
		               "path": "./github.com/Berops/claudie/services/ansibler/server/ansible-playbooks/longhorn-req.yml:2"
		           },
		           "tasks": [
		               {
		                   "hosts": {
		                       "hetzner-mh49kks-1": {
		                           "_ansible_no_log": null,
		                           "action": "wait_for_connection",
		                           "changed": false,
		                           "elapsed": 308,
		                           "failed": true,
		                           "msg": "timed out waiting for ping module test: Failed to connect to the host via ssh: ssh: connect to host 23.88.53.227 port 22: Operation timed out"
		                       },
		                       "hetzner-zi5vl5z-1": {
		                           "_ansible_no_log": null,
		                           "action": "wait_for_connection",
		                           "changed": false,
		                           "elapsed": 2
		                       }
		                   },
		                   "task": {
		                       "duration": {
		                           "end": "2023-07-10T15:41:20.232551Z",
		                           "start": "2023-07-10T15:36:11.268155Z"
		                       },
		                       "id": "56ebfbb3-6424-e6c6-fa10-000000000005",
		                       "name": "Wait 300 seconds for target connection to become reachable/usable",
		                       "path": "./github.com/Berops/claudie/services/ansibler/server/ansible-playbooks/longhorn-req.yml:8"
		                   }
		               },
		               {
		                   "hosts": {
		                       "hetzner-zi5vl5z-1": {
		                           "_ansible_no_log": null,
		                           "action": "ansible.builtin.apt",
		                           "ansible_facts": {
		                               "discovered_interpreter_python": "/usr/bin/python3"
		                           },
		                           "cache_update_time": 1689003683,
		                           "cache_updated": true,
		                           "changed": false,
		                           "invocation": {
		                               "module_args": {
		                                   "allow_change_held_packages": false,
		                                   "allow_downgrade": false,
		                                   "allow_unauthenticated": false,
		                                   "autoclean": false,
		                                   "autoremove": false,
		                                   "cache_valid_time": 0,
		                                   "clean": false,
		                                   "deb": null,
		                                   "default_release": null,
		                                   "dpkg_options": "force-confdef,force-confold",
		                                   "fail_on_autoremove": false,
		                                   "force": false,
		                                   "force_apt_get": false,
		                                   "install_recommends": null,
		                                   "lock_timeout": 60,
		                                   "name": "open-iscsi",
		                                   "only_upgrade": false,
		                                   "package": [
		                                       "open-iscsi"
		                                   ],
		                                   "policy_rc_d": null,
		                                   "purge": false,
		                                   "state": "present",
		                                   "update_cache": true,
		                                   "update_cache_retries": 5,
		                                   "update_cache_retry_max_delay": 12,
		                                   "upgrade": null
		                               }
		                           }
		                       }
		                   },
		                   "task": {
		                       "duration": {
		                           "end": "2023-07-10T15:41:23.993898Z",
		                           "start": "2023-07-10T15:41:20.252345Z"
		                       },
		                       "id": "56ebfbb3-6424-e6c6-fa10-000000000007",
		                       "name": "install open-iscsi",
		                       "path": "./go/src/github.com/Berops/claudie/services/ansibler/server/ansible-playbooks/longhorn-req.yml:14"
		                   }
		               },
		               {
		                   "hosts": {
		                       "hetzner-zi5vl5z-1": {
		                           "_ansible_no_log": null,
		                           "action": "ansible.builtin.apt",
		                           "cache_update_time": 1689003685,
		                           "cache_updated": true,
		                           "changed": false,
		                           "invocation": {
		                               "module_args": {
		                                   "allow_change_held_packages": false,
		                                   "allow_downgrade": false,
		                                   "allow_unauthenticated": false,
		                                   "autoclean": false,
		                                   "autoremove": false,
		                                   "cache_valid_time": 0,
		                                   "clean": false,
		                                   "deb": null,
		                                   "default_release": null,
		                                   "dpkg_options": "force-confdef,force-confold",
		                                   "fail_on_autoremove": false,
		                                   "force": false,
		                                   "force_apt_get": false,
		                                   "install_recommends": null,
		                                   "lock_timeout": 60,
		                                   "name": "nfs-common",
		                                   "only_upgrade": false,
		                                   "package": [
		                                       "nfs-common"
		                                   ],
		                                   "policy_rc_d": null,
		                                   "purge": false,
		                                   "state": "present",
		                                   "update_cache": true,
		                                   "update_cache_retries": 5,
		                                   "update_cache_retry_max_delay": 12,
		                                   "upgrade": null
		                               }
		                           }
		                       }
		                   },
		                   "task": {
		                       "duration": {
		                           "end": "2023-07-10T15:41:26.442875Z",
		                           "start": "2023-07-10T15:41:23.999325Z"
		                       },
		                       "id": "56ebfbb3-6424-e6c6-fa10-000000000008",
		                       "name": "install nfs-common",
		                       "path": "./github.com/Berops/claudie/services/ansibler/server/ansible-playbooks/longhorn-req.yml:20"
		                   }
		               }
		           ]
		       }
		   ],
		   "stats": {
		       "hetzner-mh49kks-1": {
		           "changed": 0,
		           "failures": 1,
		           "ignored": 0,
		           "ok": 0,
		           "rescued": 0,
		           "skipped": 0,
		           "unreachable": 0
		       },
		       "hetzner-zi5vl5z-1": {
		           "changed": 0,
		           "failures": 0,
		           "ignored": 0,
		           "ok": 3,
		           "rescued": 0,
		           "skipped": 0,
		           "unreachable": 0
		       }
		   }
		}`,
			WantErr: true,
			ErrMsg:  "hetzner-mh49kks-1 failed inside task Wait 300 seconds for target connection to become reachable/usable due to: timed out waiting for ping module test: Failed to connect to the host via ssh: ssh: connect to host 23.88.53.227 port 22: Operation timed out",
		},
		{
			Name: "test-set2",
			Input: `
		{
		    "custom_stats": {},
		    "global_custom_stats": {},
		    "plays": [
		        {
		            "play": {
		                "duration": {
		                    "end": "2023-07-11T10:38:40.518382Z",
		                    "start": "2023-07-11T10:38:26.385222Z"
		                },
		                "id": "56ebfbb3-6424-74b9-f60e-000000000003",
		                "name": "all",
		                "path": "./github.com/Berops/claudie/services/ansibler/server/ansible-playbooks/longhorn-req.yml:2"
		            },
		            "tasks": [
		                {
		                    "hosts": {
		                        "hetzner-cskeaz7-1": {
		                            "_ansible_no_log": null,
		                            "action": "wait_for_connection",
		                            "changed": false,
		                            "elapsed": 1
		                        },
		                        "hetzner-r309ten-1": {
		                            "_ansible_no_log": null,
		                            "action": "wait_for_connection",
		                            "changed": false,
		                            "elapsed": 1
		                        }
		                    },
		                    "task": {
		                        "duration": {
		                            "end": "2023-07-11T10:38:28.474580Z",
		                            "start": "2023-07-11T10:38:26.389973Z"
		                        },
		                        "id": "56ebfbb3-6424-74b9-f60e-000000000005",
		                        "name": "Wait 300 seconds for target connection to become reachable/usable",
		                        "path": "./github.com/Berops/claudie/services/ansibler/server/ansible-playbooks/longhorn-req.yml:8"
		                    }
		                },
		                {
		                    "hosts": {
		                        "hetzner-cskeaz7-1": {
		                            "_ansible_no_log": null,
		                            "action": "ansible.builtin.apt",
		                            "ansible_facts": {
		                                "discovered_interpreter_python": "/usr/bin/python3"
		                            },
		                            "cache_update_time": 1689071910,
		                            "cache_updated": true,
		                            "changed": false,
		                            "invocation": {
		                                "module_args": {
		                                    "allow_change_held_packages": false,
		                                    "allow_downgrade": false,
		                                    "allow_unauthenticated": false,
		                                    "autoclean": false,
		                                    "autoremove": false,
		                                    "cache_valid_time": 0,
		                                    "clean": false,
		                                    "deb": null,
		                                    "default_release": null,
		                                    "dpkg_options": "force-confdef,force-confold",
		                                    "fail_on_autoremove": false,
		                                    "force": false,
		                                    "force_apt_get": false,
		                                    "install_recommends": null,
		                                    "lock_timeout": 60,
		                                    "name": "open-iscsi",
		                                    "only_upgrade": false,
		                                    "package": [
		                                        "open-iscsi"
		                                    ],
		                                    "policy_rc_d": null,
		                                    "purge": false,
		                                    "state": "present",
		                                    "update_cache": true,
		                                    "update_cache_retries": 5,
		                                    "update_cache_retry_max_delay": 12,
		                                    "upgrade": null
		                                }
		                            }
		                        },
		                        "hetzner-r309ten-1": {
		                            "_ansible_no_log": null,
		                            "action": "ansible.builtin.apt",
		                            "ansible_facts": {
		                                "discovered_interpreter_python": "/usr/bin/python3"
		                            },
		                            "cache_update_time": 1689071910,
		                            "cache_updated": true,
		                            "changed": false,
		                            "invocation": {
		                                "module_args": {
		                                    "allow_change_held_packages": false,
		                                    "allow_downgrade": false,
		                                    "allow_unauthenticated": false,
		                                    "autoclean": false,
		                                    "autoremove": false,
		                                    "cache_valid_time": 0,
		                                    "clean": false,
		                                    "deb": null,
		                                    "default_release": null,
		                                    "dpkg_options": "force-confdef,force-confold",
		                                    "fail_on_autoremove": false,
		                                    "force": false,
		                                    "force_apt_get": false,
		                                    "install_recommends": null,
		                                    "lock_timeout": 60,
		                                    "name": "open-iscsi",
		                                    "only_upgrade": false,
		                                    "package": [
		                                        "open-iscsi"
		                                    ],
		                                    "policy_rc_d": null,
		                                    "purge": false,
		                                    "state": "present",
		                                    "update_cache": true,
		                                    "update_cache_retries": 5,
		                                    "update_cache_retry_max_delay": 12,
		                                    "upgrade": null
		                                }
		                            }
		                        }
		                    },
		                    "task": {
		                        "duration": {
		                            "end": "2023-07-11T10:38:31.324890Z",
		                            "start": "2023-07-11T10:38:28.486057Z"
		                        },
		                        "id": "56ebfbb3-6424-74b9-f60e-000000000007",
		                        "name": "install open-iscsi",
		                        "path": "./github.com/Berops/claudie/services/ansibler/server/ansible-playbooks/longhorn-req.yml:14"
		                    }
		                },
		                {
		                    "hosts": {
		                        "hetzner-cskeaz7-1": {
		                            "_ansible_no_log": null,
		                            "action": "ansible.builtin.apt",
		                            "cache_update_time": 1689071913,
		                            "cache_updated": true,
		                            "changed": true,
		                            "diff": {},
		                            "invocation": {
		                                "module_args": {
		                                    "allow_change_held_packages": false,
		                                    "allow_downgrade": false,
		                                    "allow_unauthenticated": false,
		                                    "autoclean": false,
		                                    "autoremove": false,
		                                    "cache_valid_time": 0,
		                                    "clean": false,
		                                    "deb": null,
		                                    "default_release": null,
		                                    "dpkg_options": "force-confdef,force-confold",
		                                    "fail_on_autoremove": false,
		                                    "force": false,
		                                    "force_apt_get": false,
		                                    "install_recommends": null,
		                                    "lock_timeout": 60,
		                                    "name": "nfs-common",
		                                    "only_upgrade": false,
		                                    "package": [
		                                        "nfs-common"
		                                    ],
		                                    "policy_rc_d": null,
		                                    "purge": false,
		                                    "state": "present",
		                                    "update_cache": true,
		                                    "update_cache_retries": 5,
		                                    "update_cache_retry_max_delay": 12,
		                                    "upgrade": null
		                                }
		                            },
		                            "stderr": "",
		                            "stderr_lines": [],
		                            "stdout": "...",
		                            "stdout_lines": [
		                                "..."
		                            ]
		                        },
		                        "hetzner-r309ten-1": {
		                            "_ansible_no_log": null,
		                            "action": "ansible.builtin.apt",
		                            "cache_update_time": 1689071912,
		                            "cache_updated": true,
		                            "changed": true,
		                            "diff": {},
		                            "invocation": {
		                                "module_args": {
		                                    "allow_change_held_packages": false,
		                                    "allow_downgrade": false,
		                                    "allow_unauthenticated": false,
		                                    "autoclean": false,
		                                    "autoremove": false,
		                                    "cache_valid_time": 0,
		                                    "clean": false,
		                                    "deb": null,
		                                    "default_release": null,
		                                    "dpkg_options": "force-confdef,force-confold",
		                                    "fail_on_autoremove": false,
		                                    "force": false,
		                                    "force_apt_get": false,
		                                    "install_recommends": null,
		                                    "lock_timeout": 60,
		                                    "name": "nfs-common",
		                                    "only_upgrade": false,
		                                    "package": [
		                                        "nfs-common"
		                                    ],
		                                    "policy_rc_d": null,
		                                    "purge": false,
		                                    "state": "present",
		                                    "update_cache": true,
		                                    "update_cache_retries": 5,
		                                    "update_cache_retry_max_delay": 12,
		                                    "upgrade": null
		                                }
		                            },
		                            "stderr": "",
		                            "stderr_lines": [],
		                            "stdout": "...,",
		                            "stdout_lines": [
		                               "..."
		                            ]
		                        }
		                    },
		                    "task": {
		                        "duration": {
		                            "end": "2023-07-11T10:38:40.518382Z",
		                            "start": "2023-07-11T10:38:31.329480Z"
		                        },
		                        "id": "56ebfbb3-6424-74b9-f60e-000000000008",
		                        "name": "install nfs-common",
		                        "path": "./github.com/Berops/claudie/services/ansibler/server/ansible-playbooks/longhorn-req.yml:20"
		                    }
		                }
		            ]
		        }
		    ],
		    "stats": {
		        "hetzner-cskeaz7-1": {
		            "changed": 1,
		            "failures": 0,
		            "ignored": 0,
		            "ok": 3,
		            "rescued": 0,
		            "skipped": 0,
		            "unreachable": 0
		        },
		        "hetzner-r309ten-1": {
		            "changed": 1,
		            "failures": 0,
		            "ignored": 0,
		            "ok": 3,
		            "rescued": 0,
		            "skipped": 0,
		            "unreachable": 0
		        }
		    }
		}`,
			WantErr: false,
		},
		{
			Name: "test-set-3",
			Input: `
		{
		   "custom_stats": {},
		   "global_custom_stats": {},
		   "plays": [
		       {
		           "play": {
		               "duration": {
		                   "end": "2023-07-11T11:15:16.542516Z",
		                   "start": "2023-07-11T11:15:16.527639Z"
		               },
		               "id": "56ebfbb3-6424-aead-3e4d-000000000004",
		               "name": "lb",
		               "path": "./github.com/Berops/claudie/services/ansibler/server/clusters/test-izwnwjx-lbs/lb-cmjuksk/node-exporter.yml:2"
		           },
		           "tasks": [
		               {
		                   "hosts": {
		                       "hetzner-fls173h-1": {
		                           "_ansible_no_log": null,
		                           "action": "ansible.builtin.fail",
		                           "changed": false,
		                           "failed": true,
		                           "msg": "The system may not be provisioned according to the CMDB status."
		                       }
		                   },
		                   "task": {
		                       "duration": {
		                           "end": "2023-07-11T11:15:16.542516Z",
		                           "start": "2023-07-11T11:15:16.533565Z"
		                       },
		                       "id": "56ebfbb3-6424-aead-3e4d-000000000008",
		                       "name": "always fail",
		                       "path": "./github.com/Berops/claudie/services/ansibler/server/clusters/test-izwnwjx-lbs/lb-cmjuksk/node-exporter.yml:7"
		                   }
		               }
		           ]
		       }
		   ],
		   "stats": {
		       "hetzner-fls173h-1": {
		           "changed": 0,
		           "failures": 1,
		           "ignored": 0,
		           "ok": 0,
		           "rescued": 0,
		           "skipped": 0,
		           "unreachable": 0
		       }
		   }
		}`,
			WantErr: true,
			ErrMsg:  "hetzner-fls173h-1 failed inside task always fail due to: The system may not be provisioned according to the CMDB status.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			err := collectErrors(strings.NewReader(tt.Input))
			if (err != nil) != tt.WantErr {
				t.Logf("collectErrors() = %v want err %v\n", err, tt.WantErr)
				return
			}
			if tt.WantErr {
				if err.Error() != tt.ErrMsg {
					t.Logf("collectErrors() - got: %s want: %s", err.Error(), tt.ErrMsg)
					return
				}
			}
		})
	}
}
