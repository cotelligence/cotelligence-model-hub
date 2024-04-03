package hub

import (
	"errors"
)

func CanModelScale(modelUUID string) (bool, error) {
	model, modelExists := GetModel(modelUUID)
	if !modelExists {
		return false, errors.New("model not found")
	}

	var pods, _ = GetAllPods()
	var notOccupiedPods []Pod
	for _, pod := range pods {
		if !pod.IsOccupied() {
			notOccupiedPods = append(notOccupiedPods, pod)
		}
	}
	var allBindings, _ = GetAllBindings()
	var alreadyOccupiedPods = 0
	// get all occupied pods by this model
	for _, binding := range allBindings {
		if binding.ModelUUID == modelUUID {
			alreadyOccupiedPods++
		}
	}

	// Check if the model can be scaled to not-occupied pods within the range of min-maxInstanceCnt
	if model.MaxInstanceCnt-alreadyOccupiedPods > 0 {
		return true, nil
	}

	return false, nil
}
