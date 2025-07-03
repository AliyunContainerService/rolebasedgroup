package utils

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	lwsv1 "sigs.k8s.io/lws/api/leaderworkerset/v1"
	"sigs.k8s.io/rbgs/test/wrappers"
	"testing"
)

func TestObjectsEqual(t *testing.T) {
	schema := runtime.NewScheme()
	appsv1.AddToScheme(schema)
	lwsv1.AddToScheme(schema)

	type args struct {
		old client.Object
		new client.Object
	}
	tests := []struct {
		name  string
		args  args
		equal bool
	}{
		{
			name: "deploy equal",
			args: args{
				old: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{
						Template: wrappers.BuildPodTemplateSpec(),
					},
				},
				new: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{
						Template: wrappers.BuildPodTemplateSpec(),
					},
				},
			},
			equal: true,
		},
		{
			name: "deploy not equal",
			args: args{
				old: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{
						Template: wrappers.BuildPodTemplateSpec(),
					},
				},
				new: &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"foo": "bar",
						},
					},
					Spec: appsv1.DeploymentSpec{
						Template: wrappers.BuildPodTemplateSpec(),
					},
				},
			},
			equal: false,
		},
		{
			name: "lws not equal",
			args: args{
				old: &lwsv1.LeaderWorkerSet{
					Spec: lwsv1.LeaderWorkerSetSpec{
						LeaderWorkerTemplate: lwsv1.LeaderWorkerTemplate{
							WorkerTemplate: wrappers.BuildPodTemplateSpec(),
						},
					},
				},
				new: &lwsv1.LeaderWorkerSet{
					Spec: lwsv1.LeaderWorkerSetSpec{
						LeaderWorkerTemplate: lwsv1.LeaderWorkerTemplate{
							WorkerTemplate: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{
											Name:  "nginx2",
											Image: "anolis-registry.cn-zhangjiakou.cr.aliyuncs.com/openanolis/nginx:1.14.1-8.6",
										},
									},
								},
							},
						},
					},
				},
			},
			equal: false,
		},
		{
			name:  "sts not equal",
			equal: false,
			args: args{
				old: &appsv1.StatefulSet{
					Spec: appsv1.StatefulSetSpec{
						Template: wrappers.BuildPodTemplateSpec(),
					},
				},
				new: &appsv1.StatefulSet{
					Spec: appsv1.StatefulSetSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name:  "nginx",
										Image: "anolis-registry.cn-zhangjiakou.cr.aliyuncs.com/openanolis/nginx:1.14.1-8.6",
										Env: []corev1.EnvVar{
											{
												Name:  "key",
												Value: "value",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "change the order of container/volume/env/volumeMount",
			args: args{
				old: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Volumes: []corev1.Volume{
									{
										Name: "volume1",
										VolumeSource: corev1.VolumeSource{
											EmptyDir: &corev1.EmptyDirVolumeSource{},
										},
									},
									{
										Name: "volume2",
										VolumeSource: corev1.VolumeSource{
											EmptyDir: &corev1.EmptyDirVolumeSource{},
										},
									},
								},
								Containers: []corev1.Container{
									{
										Name:  "nginx",
										Image: "anolis-registry.cn-zhangjiakou.cr.aliyuncs.com/openanolis/nginx:1.14.1-8.6",
										Env: []corev1.EnvVar{
											{
												Name:  "env1",
												Value: "var1",
											},
											{
												Name:  "env2",
												Value: "var2",
											},
										},
										VolumeMounts: []corev1.VolumeMount{
											{
												Name:      "volume1",
												MountPath: "/etc/nginx/nginx.conf",
											},
											{Name: "volume2",
												MountPath: "/etc/nginx2",
											},
										},
									},
									{
										Name:  "nginx2",
										Image: "anolis-registry.cn-zhangjiakou.cr.aliyuncs.com/openanolis/nginx:1.14.1-8.6",
										Env: []corev1.EnvVar{
											{
												Name:  "env1",
												Value: "var1",
											},
											{
												Name:  "env2",
												Value: "var2",
											},
										},
									},
								},
							},
						},
					},
				},
				new: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Volumes: []corev1.Volume{
									{
										Name: "volume2",
										VolumeSource: corev1.VolumeSource{
											EmptyDir: &corev1.EmptyDirVolumeSource{},
										},
									},
									{
										Name: "volume1",
										VolumeSource: corev1.VolumeSource{
											EmptyDir: &corev1.EmptyDirVolumeSource{},
										},
									},
								},
								Containers: []corev1.Container{

									{
										Name:  "nginx2",
										Image: "anolis-registry.cn-zhangjiakou.cr.aliyuncs.com/openanolis/nginx:1.14.1-8.6",
										Env: []corev1.EnvVar{
											{
												Name:  "env1",
												Value: "var1",
											},
											{
												Name:  "env2",
												Value: "var2",
											},
										},
									},
									{
										Name:  "nginx",
										Image: "anolis-registry.cn-zhangjiakou.cr.aliyuncs.com/openanolis/nginx:1.14.1-8.6",
										Env: []corev1.EnvVar{
											{
												Name:  "env2",
												Value: "var2",
											},
											{
												Name:  "env1",
												Value: "var1",
											},
										},
										VolumeMounts: []corev1.VolumeMount{
											{
												Name:      "volume2",
												MountPath: "/etc/nginx2",
											},
											{
												Name:      "volume1",
												MountPath: "/etc/nginx/nginx.conf",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			equal: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ObjectsEqual(tt.args.old, tt.args.new)
			if err != nil {
				t.Errorf("ObjectsEqual() error = %v", err)
			}
			if got != tt.equal {
				t.Errorf("ObjectsEqual() = %v, want %v", got, tt.equal)
			}
		})
	}
}
