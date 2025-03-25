package discovery

import (
	"fmt"
	"strings"

	"sigs.k8s.io/yaml"

	corev1 "k8s.io/api/core/v1"
	workloadsv1 "sigs.k8s.io/rbgs/api/workloads/v1"
)

type ConfigBuilder struct {
	RBG       *workloadsv1.RoleBasedGroup
	GroupName string
}

type ClusterConfig struct {
	Group GroupInfo `json:"group"`
	Roles RolesInfo `json:"roles"`
}

type GroupInfo struct {
	Name  string   `json:"name"`
	Size  int      `json:"size"`
	Roles []string `json:"roles"`
}

type RolesInfo map[string]RoleInstances

type RoleInstances struct {
	Size      int        `json:"size"`
	Instances []Instance `json:"instances"`
}

type Instance struct {
	Address string           `json:"address"`
	Ports   map[string]int32 `json:"ports"` // Key: port name, Value: port number
}

func (b *ConfigBuilder) Build() ([]byte, error) {
	config := ClusterConfig{
		Group: GroupInfo{
			Name:  b.GroupName,
			Size:  len(b.RBG.Spec.Roles),
			Roles: b.getRoleNames(),
		},
		Roles: b.buildRolesInfo(),
	}
	return yaml.Marshal(config)
}

func (b *ConfigBuilder) getRoleNames() []string {
	names := make([]string, 0, len(b.RBG.Spec.Roles))
	for _, r := range b.RBG.Spec.Roles {
		names = append(names, r.Name)
	}
	return names
}

func (b *ConfigBuilder) buildRolesInfo() RolesInfo {
	roles := make(RolesInfo)
	for _, role := range b.RBG.Spec.Roles {
		roles[role.Name] = RoleInstances{
			Size:      int(*role.Replicas),
			Instances: b.buildInstances(role),
		}
	}
	return roles
}

func (b *ConfigBuilder) buildInstances(role workloadsv1.RoleSpec) []Instance {
	instances := make([]Instance, 0, *role.Replicas)
	serviceName := fmt.Sprintf("%s-%s", b.GroupName, role.Name)

	for i := 0; i < int(*role.Replicas); i++ {
		instance := Instance{
			Address: fmt.Sprintf("%s-%d.%s", role.Name, i, serviceName),
			Ports:   make(map[string]int32),
		}

		// 收集所有端口
		for _, port := range role.ServicePorts {
			portName := generatePortKey(port)
			instance.Ports[portName] = port.Port
		}

		instances = append(instances, instance)
	}
	return instances
}

func generatePortKey(port corev1.ServicePort) string {
	if port.Name != "" {
		return strings.ToLower(strings.ReplaceAll(port.Name, "-", "_"))
	}
	return fmt.Sprintf("port%d", port.Port)
}
