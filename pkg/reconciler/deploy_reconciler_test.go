package reconciler

import (
	"context"
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	workloadsv1alpha1 "sigs.k8s.io/rbgs/api/workloads/v1alpha1"
)

func TestDeploymentReconciler_ConstructRoleStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme) // 注册 Deployment 类型
	_ = workloadsv1alpha1.AddToScheme(scheme)
	type fields struct {
		client client.Client
		scheme *runtime.Scheme
	}
	type args struct {
		ctx  context.Context
		rbg  *workloadsv1alpha1.RoleBasedGroup
		role *workloadsv1alpha1.RoleSpec
	}

	// 测试用 Deployment 模板
	testDeploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-rbg-test-role", // 假设 GetWorkloadName 生成此名称
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To[int32](3),
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 2,
		},
	}

	tests := []struct {
		name             string
		fields           fields
		args             args
		wantStatus       workloadsv1alpha1.RoleStatus
		wantUpdateStatus bool
		wantErr          bool
	}{
		// 用例1: Deployment存在且状态变化需要更新
		{
			name: "status-changed-needs-update",
			fields: fields{
				scheme: scheme,
				client: fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(testDeploy.DeepCopy()).
					Build(),
			},
			args: args{
				ctx: context.Background(),
				rbg: &workloadsv1alpha1.RoleBasedGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-rbg",
						Namespace: "default",
					},
					Status: workloadsv1alpha1.RoleBasedGroupStatus{
						RoleStatuses: []workloadsv1alpha1.RoleStatus{
							{
								Name:          "test-role",
								Replicas:      2, // 旧值
								ReadyReplicas: 1, // 旧值
							},
						},
					},
				},
				role: &workloadsv1alpha1.RoleSpec{Name: "test-role"},
			},
			wantStatus: workloadsv1alpha1.RoleStatus{
				Name:          "test-role",
				Replicas:      3,
				ReadyReplicas: 2,
			},
			wantUpdateStatus: true,
			wantErr:          false,
		},
		// 用例2: 状态未变化无需更新
		{
			name: "status-unchanged-no-update",
			fields: fields{
				scheme: scheme,
				client: fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(testDeploy.DeepCopy()).
					Build(),
			},
			args: args{
				ctx: context.Background(),
				rbg: &workloadsv1alpha1.RoleBasedGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-rbg",
						Namespace: "default",
					},
					Status: workloadsv1alpha1.RoleBasedGroupStatus{
						RoleStatuses: []workloadsv1alpha1.RoleStatus{
							{
								Name:          "test-role",
								Replicas:      3, // 当前值
								ReadyReplicas: 2, // 当前值
							},
						},
					},
				},
				role: &workloadsv1alpha1.RoleSpec{Name: "test-role"},
			},
			wantStatus: workloadsv1alpha1.RoleStatus{
				Name:          "test-role",
				Replicas:      3,
				ReadyReplicas: 2,
			},
			wantUpdateStatus: false,
			wantErr:          false,
		},
		// 用例3: 首次创建角色状态
		{
			name: "initial-status-creation",
			fields: fields{
				scheme: scheme,
				client: fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(testDeploy.DeepCopy()).
					Build(),
			},
			args: args{
				ctx: context.Background(),
				rbg: &workloadsv1alpha1.RoleBasedGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-rbg",
						Namespace: "default",
					},
					Status: workloadsv1alpha1.RoleBasedGroupStatus{
						RoleStatuses: []workloadsv1alpha1.RoleStatus{}, // 无现有状态
					},
				},
				role: &workloadsv1alpha1.RoleSpec{Name: "test-role"},
			},
			wantStatus: workloadsv1alpha1.RoleStatus{
				Name:          "test-role",
				Replicas:      3,
				ReadyReplicas: 2,
			},
			wantUpdateStatus: true,
			wantErr:          false,
		},
		// 用例4: Deployment不存在
		{
			name: "deployment-not-found",
			fields: fields{
				scheme: scheme,
				client: fake.NewClientBuilder().
					WithScheme(scheme).
					Build(), // 无Deployment
			},
			args: args{
				ctx: context.Background(),
				rbg: &workloadsv1alpha1.RoleBasedGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-rbg",
						Namespace: "default",
					},
					Status: workloadsv1alpha1.RoleBasedGroupStatus{
						RoleStatuses: []workloadsv1alpha1.RoleStatus{
							{Name: "test-role", Replicas: 2, ReadyReplicas: 1},
						},
					},
				},
				role: &workloadsv1alpha1.RoleSpec{Name: "test-role"},
			},
			wantStatus:       workloadsv1alpha1.RoleStatus{},
			wantUpdateStatus: false,
			wantErr:          true, // 预期返回NotFound错误
		},
		// 用例5: 副本数为零的特殊情况
		{
			name: "zero-replicas-edge-case",
			fields: fields{
				scheme: scheme,
				client: fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(&appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-rbg-test-role",
							Namespace: "default",
						},
						Spec: appsv1.DeploymentSpec{
							Replicas: ptr.To[int32](0), // 显式设置零值
						},
						Status: appsv1.DeploymentStatus{
							ReadyReplicas: 0,
						},
					}).
					Build(),
			},
			args: args{
				ctx: context.Background(),
				rbg: &workloadsv1alpha1.RoleBasedGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-rbg",
						Namespace: "default",
					},
					Status: workloadsv1alpha1.RoleBasedGroupStatus{
						RoleStatuses: []workloadsv1alpha1.RoleStatus{
							{
								Name:          "test-role",
								Replicas:      1, // 旧值非零
								ReadyReplicas: 0,
							},
						},
					},
				},
				role: &workloadsv1alpha1.RoleSpec{Name: "test-role"},
			},
			wantStatus: workloadsv1alpha1.RoleStatus{
				Name:          "test-role",
				Replicas:      0,
				ReadyReplicas: 0,
			},
			wantUpdateStatus: true, // 从1→0需要更新
			wantErr:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &DeploymentReconciler{
				scheme: tt.fields.scheme,
				client: tt.fields.client,
			}
			gotStatus, gotUpdateStatus, err := r.ConstructRoleStatus(tt.args.ctx, tt.args.rbg, tt.args.role)

			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(gotStatus, tt.wantStatus) {
				t.Errorf("gotStatus = %v, want %v", gotStatus, tt.wantStatus)
			}

			if gotUpdateStatus != tt.wantUpdateStatus {
				t.Errorf("gotUpdateStatus = %v, want %v", gotUpdateStatus, tt.wantUpdateStatus)
			}
		})
	}
}

func Test_semanticallyEqualDeployment(t *testing.T) {
	type args struct {
		old *appsv1.Deployment
		new *appsv1.Deployment
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := semanticallyEqualDeployment(tt.args.dep1, tt.args.dep2); got != tt.want {
				t.Errorf("semanticallyEqualDeployment() = %v, want %v", got, tt.want)
			}
		})
	}
}
