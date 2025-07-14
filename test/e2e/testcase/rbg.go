package testcase

import (
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	workloadsv1alpha1 "sigs.k8s.io/rbgs/api/workloads/v1alpha1"
	"sigs.k8s.io/rbgs/test/e2e/framework"
	"sigs.k8s.io/rbgs/test/utils"
	"sigs.k8s.io/rbgs/test/wrappers"
)

func RunRbgControllerTestCases(f *framework.Framework) {
	ginkgo.Describe("rbg controller", func() {

		ginkgo.It("create & delete rbg with multi workloads", func() {
			rbg := wrappers.BuildBasicRoleBasedGroup("e2e-test", f.Namespace).
				WithRoles(
					[]workloadsv1alpha1.RoleSpec{
						wrappers.BuildBasicRole("role-deploy").WithWorkload(workloadsv1alpha1.DeploymentWorkloadType).Obj(),
						wrappers.BuildBasicRole("role-sts").WithWorkload(workloadsv1alpha1.StatefulSetWorkloadType).Obj(),
						wrappers.BuildLwsRole("role-lws").Obj(),
					}).Obj()

			gomega.Expect(f.Client.Create(f.Ctx, rbg)).Should(gomega.Succeed())

			// delete rbg
			gomega.Expect(f.Client.Delete(f.Ctx, rbg)).Should(gomega.Succeed())
			f.ExpectRbgDeleted(rbg)
		})

		ginkgo.It("rbg with dependency", func() {
			rbg := wrappers.BuildBasicRoleBasedGroup("e2e-test", f.Namespace).
				WithRoles(
					[]workloadsv1alpha1.RoleSpec{
						wrappers.BuildBasicRole("role-1").WithWorkload(workloadsv1alpha1.StatefulSetWorkloadType).Obj(),
						wrappers.BuildBasicRole("role-2").WithWorkload(workloadsv1alpha1.StatefulSetWorkloadType).
							WithDependencies([]string{"role-1"}).Obj(),
					}).Obj()

			gomega.Expect(f.Client.Create(f.Ctx, rbg)).Should(gomega.Succeed())

			f.ExpectRbgEqual(rbg)
		})

		ginkgo.It("rbg with engine runtime existed", func() {
			rbg := wrappers.BuildBasicRoleBasedGroup("e2e-test", f.Namespace).
				WithRoles(
					[]workloadsv1alpha1.RoleSpec{
						wrappers.BuildBasicRole("role-1").
							WithWorkload(workloadsv1alpha1.StatefulSetWorkloadType).
							WithEngineRuntime(
								[]workloadsv1alpha1.EngineRuntime{{ProfileName: utils.DefaultEngineRuntimeProfileName}}).
							Obj(),
					}).Obj()

			gomega.Expect(utils.CreatePatioRuntime(f.Ctx, f.Client)).Should(gomega.Succeed())

			gomega.Expect(f.Client.Create(f.Ctx, rbg)).Should(gomega.Succeed())

			f.ExpectRbgEqual(rbg)
		})

		ginkgo.It("rbg with orphan roles", func() {
			rbg := wrappers.BuildBasicRoleBasedGroup("e2e-test", f.Namespace).WithRoles(
				[]workloadsv1alpha1.RoleSpec{
					wrappers.BuildBasicRole("role-1").Obj(),
					wrappers.BuildBasicRole("role-2").Obj(),
				}).Obj()
			gomega.Expect(f.Client.Create(f.Ctx, rbg)).Should(gomega.Succeed())
			f.ExpectRbgEqual(rbg)

			// update role name
			utils.UpdateRbg(f.Ctx, f.Client, rbg, func(rbg *workloadsv1alpha1.RoleBasedGroup) {
				rbg.Spec.Roles = []workloadsv1alpha1.RoleSpec{
					wrappers.BuildBasicRole("sts-1").Obj(),
					wrappers.BuildBasicRole("sts-2").Obj(),
				}
			})
			f.ExpectRbgEqual(rbg)

			f.ExpectWorkloadNotExist(rbg, wrappers.BuildBasicRole("role-1").Obj())
			f.ExpectWorkloadNotExist(rbg, wrappers.BuildBasicRole("role-2").Obj())
		})

	})

}
