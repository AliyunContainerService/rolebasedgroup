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

	// SetRoleIndexLabelKey identifies pod's position in role replica set
	// Value: Zero-padded numeric index (e.g., "001", "002")
	SetRoleIndexLabelKey = RBGDomainPrefix + "role-index"

	// RevisionAnnotationKey tracks the controller revision hash for template changes
	// Value: SHA256 hash of RoleSpec template
	RevisionAnnotationKey = RBGDomainPrefix + "revision"

	RoleSizeAnnotationKey string = RBGDomainPrefix + "role-size"

	GroupSizeAnnotationKey string = RBGDomainPrefix + "group-size"
)

type AdapterPhase string

const (
	AdapterPhaseNone     AdapterPhase = ""
	AdapterPhaseNotBound AdapterPhase = "NotBound"
	AdapterPhaseBound    AdapterPhase = "Bound"
)
