package testcase

import (
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	workloadsv1alpha1 "sigs.k8s.io/rbgs/api/workloads/v1alpha1"
	"sigs.k8s.io/rbgs/test/e2e/framework"
	"sigs.k8s.io/rbgs/test/utils"
	"sigs.k8s.io/rbgs/test/wrappers"
)

func RunDeploymentWorkloadTestCases(f *framework.Framework) {
	ginkgo.It("update deploy role.replicas & role.Template", func() {
		rbg := wrappers.BuildBasicRoleBasedGroup("e2e-test", f.Namespace).WithRoles([]workloadsv1alpha1.RoleSpec{
			wrappers.BuildBasicRole("role-1").WithWorkload(workloadsv1alpha1.DeploymentWorkloadType).Obj(),
		}).Obj()
		gomega.Expect(f.Client.Create(f.Ctx, rbg)).Should(gomega.Succeed())
		f.ExpectRbgEqual(rbg)

		updateLabel := map[string]string{"update-label": "new"}
		utils.UpdateRbg(f.Ctx, f.Client, rbg, func(rbg *workloadsv1alpha1.RoleBasedGroup) {
			rbg.Spec.Roles[0].Replicas = ptr.To(*rbg.Spec.Roles[0].Replicas + 1)
			rbg.Spec.Roles[0].Template.Labels = updateLabel
		})
		f.ExpectRbgEqual(rbg)

		f.ExpectWorkloadLabelContains(rbg, rbg.Spec.Roles[0], updateLabel)
	})

	ginkgo.It("deploy with rollingUpdate", func() {
		rbg := wrappers.BuildBasicRoleBasedGroup("e2e-test", f.Namespace).WithRoles(
			[]workloadsv1alpha1.RoleSpec{
				wrappers.BuildBasicRole("role-1").
					WithReplicas(2).
					WithWorkload(workloadsv1alpha1.DeploymentWorkloadType).
					WithRollingUpdate(workloadsv1alpha1.RollingUpdate{
						MaxUnavailable: intstr.FromInt32(1),
						MaxSurge:       intstr.FromInt32(1),
					}).Obj(),
			}).Obj()
		gomega.Expect(f.Client.Create(f.Ctx, rbg)).Should(gomega.Succeed())
		f.ExpectRbgEqual(rbg)

		// update, start rolling update
		updateLabel := map[string]string{"update-label": "new"}
		utils.UpdateRbg(f.Ctx, f.Client, rbg, func(rbg *workloadsv1alpha1.RoleBasedGroup) {
			rbg.Spec.Roles[0].Template.Labels = updateLabel
		})
		f.ExpectRbgEqual(rbg)

		f.ExpectWorkloadLabelContains(rbg, rbg.Spec.Roles[0], updateLabel)

	})

	ginkgo.It("deploy with restartPolicy", func() {
		rbg := wrappers.BuildBasicRoleBasedGroup("e2e-test", f.Namespace).WithRoles(
			[]workloadsv1alpha1.RoleSpec{
				wrappers.BuildBasicRole("role-1").
					WithReplicas(2).
					WithWorkload(workloadsv1alpha1.DeploymentWorkloadType).
					WithRestartPolicy(workloadsv1alpha1.RecreateRBGOnPodRestart).
					Obj(),
			}).Obj()
		gomega.Expect(f.Client.Create(f.Ctx, rbg)).Should(gomega.Succeed())
		f.ExpectRbgEqual(rbg)

		gomega.Expect(utils.DeletePod(f.Ctx, f.Client, f.Namespace, rbg.Name)).Should(gomega.Succeed())

		// wait rbg recreate
		f.ExpectRbgEqual(rbg)
		f.ExpectRbgCondition(rbg, workloadsv1alpha1.RoleBasedGroupRestartInProgress, metav1.ConditionFalse)
	})

}
