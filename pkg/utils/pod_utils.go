package utils

import (
	corev1 "k8s.io/api/core/v1"
	"sort"
)

// PodRunningAndReady checks if the pod condition is running and marked as ready.
func PodRunningAndReady(pod corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodRunning && podReady(pod)
}

func podReady(pod corev1.Pod) bool {
	return podReadyConditionTrue(pod.Status)
}

func podReadyConditionTrue(status corev1.PodStatus) bool {
	condition := getPodReadyCondition(status)
	return condition != nil && condition.Status == corev1.ConditionTrue
}

func getPodReadyCondition(status corev1.PodStatus) *corev1.PodCondition {
	_, condition := getPodCondition(&status, corev1.PodReady)
	return condition
}
func getPodCondition(status *corev1.PodStatus, conditionType corev1.PodConditionType) (int, *corev1.PodCondition) {
	if status == nil {
		return -1, nil
	}
	return getPodConditionFromList(status.Conditions, conditionType)
}

func getPodConditionFromList(conditions []corev1.PodCondition, conditionType corev1.PodConditionType) (int, *corev1.PodCondition) {
	if conditions == nil {
		return -1, nil
	}
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return i, &conditions[i]
		}
	}
	return -1, nil
}

func SortPodSpec(podSpec corev1.PodSpec) {
	// sort Volumes
	sort.Slice(podSpec.Volumes, func(i, j int) bool {
		return podSpec.Volumes[i].Name < podSpec.Volumes[j].Name
	})

	// sort InitContainers
	for i := range podSpec.InitContainers {
		sortContainer(podSpec.InitContainers[i])
	}
	sort.Slice(podSpec.InitContainers, func(i, j int) bool {
		return podSpec.InitContainers[i].Name < podSpec.InitContainers[j].Name
	})

	// sort Containers
	for i := range podSpec.Containers {
		sortContainer(podSpec.Containers[i])
	}
	sort.Slice(podSpec.Containers, func(i, j int) bool {
		return podSpec.Containers[i].Name < podSpec.Containers[j].Name
	})
}

func sortContainer(c corev1.Container) {
	// sort Env
	sort.Slice(c.Env, func(i, j int) bool {
		return c.Env[i].Name < c.Env[j].Name
	})
	// sort VolumeMount
	sort.Slice(c.VolumeMounts, func(i, j int) bool {
		return c.VolumeMounts[i].Name < c.VolumeMounts[j].Name
	})
}

// ContainerRestarted return true when there is any container in the pod that gets restarted
func ContainerRestarted(pod *corev1.Pod) bool {
	if pod.Status.Phase == corev1.PodRunning || pod.Status.Phase == corev1.PodPending {
		for j := range pod.Status.InitContainerStatuses {
			stat := pod.Status.InitContainerStatuses[j]
			if stat.RestartCount > 0 {
				return true
			}
		}
		for j := range pod.Status.ContainerStatuses {
			// if engine runtime restart, do not need to recreate rbg.
			if pod.Status.ContainerStatuses[j].Name == "patio-runtime" {
				continue
			}
			stat := pod.Status.ContainerStatuses[j]
			if stat.RestartCount > 0 {
				return true
			}
		}
	}
	return false
}

// PodDeleted checks if the worker pod has been deleted
func PodDeleted(pod *corev1.Pod) bool {
	return pod.DeletionTimestamp != nil
}
