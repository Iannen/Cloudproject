package config

import (
	"time"
)

const (
	Timeout        = 3 * time.Second
	NodeNamePrefix = "kaffcloud"
)

type RoleSpec struct {
	Name     string
	Replicas int
}

var ClusterSpec = []RoleSpec{
	{Name: "tailscale-manager", Replicas: 1},
}
