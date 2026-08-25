package config

import (
	"go-controller/src/core/models"
	"time"
)

const (
	Timeout        = 3 * time.Second
	NodeNamePrefix = "kaffcloud"
)

var ClusterSpec = []models.RoleSpec{
	{Name: "tailscale-manager", Replicas: 1},
}
