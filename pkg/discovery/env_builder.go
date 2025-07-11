package discovery

import (
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	workloadsv1alpha1 "sigs.k8s.io/rbgs/api/workloads/v1alpha1"
)

type EnvBuilder struct {
	rbg  *workloadsv1alpha1.RoleBasedGroup
	role *workloadsv1alpha1.RoleSpec
}

func (g *EnvBuilder) Build() []corev1.EnvVar {
	envMap := make(map[string]corev1.EnvVar)

	for _, env := range g.buildLocalRoleVars() {
		envMap[env.Name] = env
	}
	if sizeVar := g.buildRoleSizeVar(); sizeVar.Name != "" {
		envMap[sizeVar.Name] = sizeVar
	}
	if groupSizeVar := g.buildGroupSizeVar(); groupSizeVar.Name != "" {
		envMap[groupSizeVar.Name] = groupSizeVar
	}

	// address env数量比较多并且可以通过以上env拼凑出来，暂不添加address env
	//for _, env := range g.buildRoleAddressVars() {
	//	envMap[env.Name] = env
	//}

	envVars := make([]corev1.EnvVar, 0, len(envMap))
	for _, env := range envMap {
		envVars = append(envVars, env)
	}

	sort.Slice(envVars, func(i, j int) bool {
		return envVars[i].Name < envVars[j].Name
	})
	return envVars
}

func (g *EnvBuilder) buildRoleSizeVar() corev1.EnvVar {
	return corev1.EnvVar{
		Name:  fmt.Sprintf("ROLES_%s_SIZE", strings.ToUpper(g.role.Name)),
		Value: fmt.Sprintf("%d", *g.role.Replicas),
	}
}

func (g *EnvBuilder) buildGroupSizeVar() corev1.EnvVar {
	groupSize := 0
	for _, role := range g.rbg.Spec.Roles {
		groupSize += int(*role.Replicas)
	}
	return corev1.EnvVar{
		Name:  "RBG_GROUP_SIZE",
		Value: fmt.Sprintf("%d", groupSize),
	}
}

func (g *EnvBuilder) buildRoleAddressVars() []corev1.EnvVar {
	var envVars []corev1.EnvVar
	for _, role := range g.rbg.Spec.Roles {
		serviceName := fmt.Sprintf("%s-%s", g.rbg.Name, role.Name)
		for i := 0; i < int(*role.Replicas); i++ {
			basePrefix := fmt.Sprintf("ROLES_%s_%d", strings.ToUpper(role.Name), i)

			// 地址变量
			envVars = append(envVars, corev1.EnvVar{
				Name:  basePrefix + "_ADDRESS",
				Value: fmt.Sprintf("%s-%d.%s", role.Name, i, serviceName),
			})

			// 多端口变量
			// # 基础地址
			// ROLES_PREFILL_0_ADDRESS=prefill-0.deepseek-0-prefill

			// # 多端口变量（假设 ServicePorts 定义如下）：
			// # - name: http, port: 8080
			// # - name: metrics, port: 9090
			// ROLES_PREFILL_0_HTTP_PORT=8080
			// ROLES_PREFILL_0_METRICS_PORT=9090

			for _, port := range role.ServicePorts {
				portKey := generatePortKey(port)
				envVars = append(envVars,
					corev1.EnvVar{
						Name:  fmt.Sprintf("%s_%s_PORT", basePrefix, strings.ToUpper(portKey)),
						Value: fmt.Sprintf("%d", port.Port),
					},
				)
			}
		}
	}
	return envVars
}

func (g *EnvBuilder) buildLocalRoleVars() []corev1.EnvVar {

	// Inject environment variables for service discovery
	envVars := []corev1.EnvVar{
		{
			Name:  "GROUP_NAME",
			Value: g.rbg.Name,
		},
		{
			Name:  "ROLE_NAME",
			Value: g.role.Name,
		},
	}

	if g.role.Workload.Kind == "StatefulSet" || g.role.Workload.Kind == "LeaderWorkerSet" {
		envVars = append(envVars,
			corev1.EnvVar{
				Name: "ROLE_INDEX",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.labels['apps.kubernetes.io/pod-index']",
					},
				},
			})
	}

	return envVars
}
