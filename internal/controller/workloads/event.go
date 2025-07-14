package workloads

// rbg-controller events
const (
	FailedGetRBG              = "FailedGetRBG"
	InvalidRoleDependency     = "InvalidRoleDependency"
	FailedCheckRoleDependency = "FailedCheckRoleDependency"
	FailedReconcileWorkload   = "FailedReconcileWorkload"
	Succeed                   = "Succeed"
	FailedUpdateStatus        = "FailedUpdateStatus"
	FailedCreatePodGroup      = "FailedCreatePodGroup"
)

// rbg-scaling-adapter events
const (
	SuccessfulBound            = "SuccessfulBound"
	SuccessfulScale            = "SuccessfulScale"
	FailedScale                = "FailedScale"
	FailedGetRBGRole           = "FailedGetRBGRole"
	FailedGetRBGScalingAdapter = "FailedGetRBGScalingAdapter"
)
