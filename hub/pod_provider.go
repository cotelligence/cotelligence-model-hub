package hub

import (
	"cotelligence-model-hub/log"

	"go.uber.org/zap"
)

type InsufficientGPUsError struct {
	Message string
}

func (e *InsufficientGPUsError) Error() string {
	return e.Message
}

type PodProviderAPI interface {
	ListPods() ([]Pod, error)
	CreatePod(imageURL string) (Pod, error)
	EditPod(podID, imageURL string) error
	RemovePod(podID string) error
	StopPod(podID string) error
	WaitForAPIReady(podID string) error
	ResumePod(podId string) error
}

func SyncPods(podProviderAPI PodProviderAPI) error {
	newPodList, err := podProviderAPI.ListPods()
	if err != nil {
		log.ZapLogger.Error("Failed to retrieve pod list", zap.Error(err))
		return err
	}

	existingPodMap, err := GetAllPodIds()
	if err != nil {
		log.ZapLogger.Error("Failed to get existing pods from cache", zap.Error(err))
		return err
	}

	// Add new pods and update existing ones
	for _, newPod := range newPodList {
		if _, found := existingPodMap[newPod.ID]; !found {
			log.ZapLogger.Info("Adding new pod", zap.String("podID", newPod.ID))
			err := AddPod(newPod)
			if err != nil {
				log.ZapLogger.Error("Failed to add new pod to cache", zap.Error(err))
			}
		} else {
			// clean up cold pod and mark it as not up
			podInRedis := existingPodMap[newPod.ID]
			if newPod.IsPodUp && !podInRedis.IsOccupied() {
				// TODO: remove this after demo
				// pod with image name: runpod/tensorflow should not be stopped
				if newPod.Image != "runpod/tensorflow" {
					log.ZapLogger.Info("Stopping cold pod", zap.String("podID", newPod.ID))
					err = podProviderAPI.StopPod(newPod.ID)
					newPod.IsPodUp = false
					if err != nil {
						log.ZapLogger.Error("Failed to stop cold pod", zap.Error(err))
					}
				}
			}
			newPod.LastUsed = podInRedis.LastUsed
			// Update existing pod
			log.ZapLogger.Info("Updating existing pod", zap.String("podID", newPod.ID))
			err = ModifyPod(newPod)
			if err != nil {
				log.ZapLogger.Error("Failed to update pod image", zap.Error(err))
			}
			delete(existingPodMap, newPod.ID)
		}

	}

	// Remove non-existing pods
	for podID := range existingPodMap {
		log.ZapLogger.Info("Deleting non-existing pod", zap.String("podID", podID))
		err := RemovePod(podID)
		if err != nil {
			log.ZapLogger.Error("Failed to remove pod from cache", zap.Error(err))
		}
	}
	return nil
}
