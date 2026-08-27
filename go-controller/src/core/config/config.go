package config

type RoleSpec struct {
	Name     string
	Replicas int
}

var ClusterSpec = []RoleSpec{
	{Name: "tailscale-manager", Replicas: 1},
}
