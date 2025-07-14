package v1alpha1

const (
	ControllerName = "rolebasedgroup-controller"
	// Domain prefix for all labels/annotations to avoid conflicts
	RBGDomainPrefix = "rolebasedgroup.workloads.x-k8s.io/"

	// SetNameLabelKey identifies resources belonging to a specific RoleBasedGroup
	// Value: RoleBasedGroup.metadata.name
	SetNameLabelKey = RBGDomainPrefix + "name"

	// SetRoleLabelKey identifies resources belonging to a specific role
	// Value: RoleSpec.name from RoleBasedGroup.spec.roles[]
	SetRoleLabelKey = RBGDomainPrefix + "role"

	// RevisionAnnotationKey tracks the controller revision hash for template changes
	// Value: SHA256 hash of RoleSpec template
	RevisionAnnotationKey = RBGDomainPrefix + "revision"

	RoleSizeAnnotationKey string = RBGDomainPrefix + "role-size"
)

type RolloutStrategyType string

const (
	// RollingUpdateStrategyType indicates that replicas will be updated one by one(defined
	// by RollingUpdateConfiguration), the latter one will not start the update until the
	// former role is ready.
	RollingUpdateStrategyType RolloutStrategyType = "RollingUpdate"
)

type RestartPolicyType string

const (
	// None will follow the same behavior as the StatefulSet/Deployment where only the failed pod
	// will be restarted on failure and other pods in the group will not be impacted.
	NoneRestartPolicy RestartPolicyType = "None"

	// RecreateRBGOnPodRestart will recreate all the pods in the rbg if
	// 1. Any individual pod in the rbg is recreated; 2. Any containers/init-containers
	// in a pod is restarted. This is to ensure all pods/containers in the group will be
	// started in the same time.
	RecreateRBGOnPodRestart RestartPolicyType = "RecreateRBGOnPodRestart"

	//RecreateRoleInstanceOnPodRestart will recreate an instance of role. If role's workload is lws, it means when a pod
	// failed, we will recreate only one lws instance, not all lws instances.
	// It equals to RecreateGroupOnPodRestart in lws.spec.LeaderWorkerTemplate.RestartPolicyType
	RecreateRoleInstanceOnPodRestart RestartPolicyType = "RecreateRoleInstanceOnPodRestart"
)

const (
	DeploymentWorkloadType      string = "apps/v1/Deployment"
	StatefulSetWorkloadType     string = "apps/v1/StatefulSet"
	LeaderWorkerSetWorkloadType string = "leaderworkerset.x-k8s.io/v1/LeaderWorkerSet"
)
