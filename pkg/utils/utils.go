package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"hash"
	"hash/fnv"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	lwsv1 "sigs.k8s.io/lws/api/leaderworkerset/v1"
)

const (
	FieldManager = "rbg"

	PatchAll    PatchType = "all"
	PatchSpec   PatchType = "spec"
	PatchStatus PatchType = "status"
)

type PatchType string

func PatchObjectApplyConfiguration(ctx context.Context, k8sClient client.Client, objApplyConfig interface{}, patchType PatchType) error {
	logger := log.FromContext(ctx)
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(objApplyConfig)
	if err != nil {
		logger.Error(err, "Converting obj apply configuration to json.")
		return err
	}

	patch := &unstructured.Unstructured{
		Object: obj,
	}

	logger.V(1).Info("patch content", "patchObject", patch.Object)

	// Use server side apply and add fieldmanager to the rbg owned fields
	// If there are conflicts in the fields owned by the rbg controller, rbg will obtain the ownership and force override
	// these fields to the ones desired by the rbg controller
	// TODO b/316776287 add E2E test for SSA
	if patchType == PatchSpec || patchType == PatchAll {
		err = k8sClient.Patch(ctx, patch, client.Apply, &client.PatchOptions{
			FieldManager: FieldManager,
			Force:        ptr.To[bool](true),
		})
		if err != nil {
			logger.Error(err, "Using server side apply to patch object")
			return err
		}
	}

	if patchType == PatchStatus || patchType == PatchAll {
		err = k8sClient.Status().Patch(ctx, patch, client.Apply,
			&client.SubResourcePatchOptions{
				PatchOptions: client.PatchOptions{
					FieldManager: FieldManager,
					Force:        ptr.To[bool](true),
				},
			})
		if err != nil {
			logger.Error(err, "Using server side apply to patch object status")
			return err
		}
	}

	return nil
}

func ContainsString(slice []string, str string) bool {
	for _, v := range slice {
		if v == str {
			return true
		}
	}
	return false
}

func PrettyJson(object interface{}) string {
	b, err := json.MarshalIndent(object, "", "    ")
	if err != nil {
		fmt.Printf("ERROR: PrettyJson, %v\n %s\n", err, b)
		return ""
	}
	return string(b)
}

// DumpJSON returns the JSON encoding
func DumpJSON(o interface{}) string {
	j, _ := json.Marshal(o)
	return string(j)
}

func NonZeroValue(value int32) int32 {
	if value < 0 {
		return 0
	}
	return value
}

func ObjectsEqual(old, new client.Object) (bool, error) {
	if old == nil || new == nil {
		return old == new, nil
	}

	oldCopy, newCopy := old.DeepCopyObject(), new.DeepCopyObject()
	var oldSpec, newSpec interface{}
	switch oldCopy.(type) {
	case *appsv1.Deployment:
		oldDeploy, newDeploy := oldCopy.(*appsv1.Deployment), newCopy.(*appsv1.Deployment)
		SortPodSpec(oldDeploy.Spec.Template.Spec)
		SortPodSpec(newDeploy.Spec.Template.Spec)
		oldSpec, newSpec = oldDeploy.Spec, newDeploy.Spec
	case *appsv1.StatefulSet:
		oldSts, newSts := oldCopy.(*appsv1.StatefulSet), newCopy.(*appsv1.StatefulSet)
		SortPodSpec(oldSts.Spec.Template.Spec)
		SortPodSpec(newSts.Spec.Template.Spec)
		oldSpec, newSpec = oldSts.Spec, newSts.Spec
	case *lwsv1.LeaderWorkerSet:
		oldLws, newLws := oldCopy.(*lwsv1.LeaderWorkerSet), newCopy.(*lwsv1.LeaderWorkerSet)
		if oldLws.Spec.LeaderWorkerTemplate.LeaderTemplate != nil {
			SortPodSpec(oldLws.Spec.LeaderWorkerTemplate.LeaderTemplate.Spec)
		}
		if newLws.Spec.LeaderWorkerTemplate.LeaderTemplate != nil {
			SortPodSpec(newLws.Spec.LeaderWorkerTemplate.LeaderTemplate.Spec)
		}
		SortPodSpec(oldLws.Spec.LeaderWorkerTemplate.WorkerTemplate.Spec)
		SortPodSpec(newLws.Spec.LeaderWorkerTemplate.WorkerTemplate.Spec)
		oldSpec, newSpec = oldLws.Spec, newLws.Spec
	default:
		return false, fmt.Errorf("unsupported object type: %T", old)

	}

	oldHash := ComputeHash(old.GetAnnotations(), old.GetLabels(), oldSpec)
	newHash := ComputeHash(new.GetAnnotations(), new.GetLabels(), newSpec)

	return oldHash == newHash, nil
}

// ComputeHash returns a hash value calculated from pod template and
// a collisionCount to avoid hash collision. The hash will be safe encoded to
// avoid bad words.
// are adapted from kubernetes/pkg/controller/controller_utils.go
func ComputeHash(objects ...interface{}) string {
	hasher := fnv.New32a()
	for _, obj := range objects {
		deepHashObject(hasher, obj)
	}

	return rand.SafeEncodeString(fmt.Sprint(hasher.Sum32()))
}

func deepHashObject(hasher hash.Hash, objectToWrite interface{}) {
	printer := spew.ConfigState{
		Indent:                  " ",
		SortKeys:                true,
		DisableMethods:          true,
		SpewKeys:                true,
		DisablePointerAddresses: true,
		DisableCapacities:       true,
	}
	_, err := printer.Fprintf(hasher, "%#v", objectToWrite)
	if err != nil {
		return
	}
}
