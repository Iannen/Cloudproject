package config

import (
	"go-controller/src/core/models"
	"time"
)

const (
	PrefixHeartbeats = "heartbeats/nodes/"
	PrefixDefs       = "assignments/definitions/"
)

const (
	AssimilateURLPattern = "http://%s:8080/assimilate"
	ActivateURLPattern   = "http://%s:8080/activate"
	EtcdEndpoint         = "localhost:2379"
	EtcdPeerPort         = 2380
	NodeNamePrefix       = "kaffcloud"
	BootstrapDir         = "/root/bootstrap"
)

const (
	Timeout         = 3 * time.Second
	StartupRetries  = 10
	StartupInterval = 1 * time.Second
)

var ClusterSpec = []models.RoleSpec{
	{Name: "tailscale-manager", Replicas: 1},
}
