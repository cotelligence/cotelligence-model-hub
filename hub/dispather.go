package hub

import (
	"cotelligence-model-hub/chain"
	"cotelligence-model-hub/log"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/google/uuid"
)

const namespaceUUIDStr = "12345678-abcd-bcde-1234-123456321123"
const HotOccupiedMinutes = 10

var dispatcherDaemonOnce sync.Once

func RegisterModel(body Model) (Model, error) {

	// Define a namespace UUID (can be any UUID)
	namespaceUUID := uuid.Must(uuid.Parse(namespaceUUIDStr))

	// Generate a name-based UUID using the unique key (in this case, the name)s
	newUUID := uuid.NewSHA1(namespaceUUID, []byte(body.Name)).String()

	model := Model{Name: body.Name, ImageURL: body.ImageURL, UUID: newUUID, MinInstanceCnt: body.MinInstanceCnt, MaxInstanceCnt: body.MaxInstanceCnt, Type: body.Type}
	err := AddModel(model)
	if err != nil {
		return Model{}, err
	}
	return model, nil
}

func DeployModelToPod(modelUUID string, runPodAPI PodProviderAPI) (*Pod, error) {
	// check model existance
	model, modelExists := GetModel(modelUUID)
	if !modelExists {
		return nil, errors.New("model not found")
	}
	var modelImage = model.ImageURL
	// Separate pods into two groups
	var notOccupiedPods []Pod
	// not-occupied pods with the same model image
	var sameImagePods []Pod
	var sameModelPods []Pod
	var pods, _ = GetAllPods()
	for _, pod := range pods {
		binding, exists := GetModelPodBinding(pod.ID)
		if !pod.IsOccupied() {
			if pod.Image == modelImage {
				sameImagePods = append(sameImagePods, pod)
			} else {
				notOccupiedPods = append(notOccupiedPods, pod)
			}
		} else if exists && binding.ModelUUID == modelUUID {
			sameModelPods = append(sameModelPods, pod)
		}
	}

	// Prioritize pods with the same model binding, then not-occupied pods with the same image
	selectedPod := roundRobinPod(sameModelPods)
	if selectedPod == nil {
		selectedPod = roundRobinPod(sameImagePods)
		if selectedPod == nil {
			selectedPod = roundRobinPod(notOccupiedPods)
		}
	} else {
		// use existing pod
		log.ZapLogger.Info("Pod already occupied by the same model, extend occupation time", zap.String("podID", selectedPod.ID))
		// Extend the occupation time
		selectedPod.LastUsed = time.Now()
		err := ModifyPod(*selectedPod)
		if err != nil {
			return nil, err
		}
		return selectedPod, nil
	}

	// scale model to new pod
	// Check if the model can be scaled
	canScale, err := CanModelScale(modelUUID)
	if err != nil {
		log.ZapLogger.Error("Failed to check if model can be scaled", zap.Error(err))
		return nil, err
	}
	if !canScale {
		return nil, errors.New("model cannot be scaled")
	}

	// if no more available not-occupied pod, try to create a new one
	if selectedPod == nil && len(pods) < chain.MaxPodsCnt {
		log.ZapLogger.Info("Create a new pod", zap.String("modelUUID", modelUUID))
		newPod, err := runPodAPI.CreatePod(model.ImageURL)
		// trigger syncPods immediately
		go SyncPods(runPodAPI)

		if err != nil {
			return nil, err
		}
		selectedPod = &newPod
	}

	// Scale the model to the selected pod
	log.ZapLogger.Info("Scale model to pod", zap.String("modelUUID", modelUUID), zap.String("podID", selectedPod.ID))
	if err := runPodAPI.EditPod(selectedPod.ID, model.ImageURL); err != nil {
		return nil, err
	}
	// resume the pod if it's down
	if !selectedPod.IsPodUp {
		err = runPodAPI.ResumePod(selectedPod.ID)
		if err != nil {
			// if the error is InsufficientGPUsError, we should delete the pod and create a new one
			var insufficientGPUsError *InsufficientGPUsError
			if errors.As(err, &insufficientGPUsError) {
				log.ZapLogger.Error("Insufficient GPUs, delete and create a new pod", zap.String("podID", selectedPod.ID))
				log.ZapLogger.Info("Remove pod", zap.String("podID", selectedPod.ID))
				err = runPodAPI.RemovePod(selectedPod.ID)
				if err != nil {
					return nil, err
				}
				log.ZapLogger.Info("Create a new pod", zap.String("modelUUID", modelUUID))
				newPod, err := runPodAPI.CreatePod(model.ImageURL)
				// trigger syncPods immediately
				go SyncPods(runPodAPI)

				if err != nil {
					return nil, err
				}
				selectedPod = &newPod
			} else {
				return nil, err
			}
		}
	}

	// Mark the pod as occupied first to avoid race condition and set the occupied until timestamp
	selectedPod.LastUsed = time.Now()
	err = ModifyPod(*selectedPod)
	log.ZapLogger.Info("Mark pod as occupied", zap.String("podID", selectedPod.ID))
	if err != nil {
		return nil, err
	}

	// Record the new model-pod binding
	err = BindModelToPod(ModelPodBinding{ModelUUID: modelUUID, PodID: selectedPod.ID})
	if err != nil {
		return nil, err
	}

	log.ZapLogger.Info("Waiting for API to be ready", zap.String("podID", selectedPod.ID))

	if err := runPodAPI.WaitForAPIReady(selectedPod.ID); err != nil {
		return nil, err
	}
	log.ZapLogger.Info("API is ready", zap.String("podID", selectedPod.ID))

	return selectedPod, nil
}

func GetPredictionEndPoint(modelUUID string, runPodAPI PodProviderAPI) (string, string, error) {
	selectedPod, err := DeployModelToPod(modelUUID, runPodAPI)
	if err != nil {
		return "", "", err
	}
	// Return the API endpoint for the prediction
	predictionAPIEndpoint := fmt.Sprintf("https://%s-5000.proxy.runpod.net/predictions", selectedPod.ID)
	return selectedPod.ID, predictionAPIEndpoint, nil
}

func roundRobinPod(pods []Pod) *Pod {
	if len(pods) == 0 {
		return nil
	}

	// Sort the pods by the last used time
	sort.Slice(pods, func(i, j int) bool {
		return pods[i].LastUsed.Before(pods[j].LastUsed)
	})

	// Select the pod that hasn't been used for the longest time
	selectedPod := pods[0]

	return &selectedPod
}

func init() {
	StartUnbindingDaemon()
}

func StartUnbindingDaemon() {
	dispatcherDaemonOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				bindings, _ := GetAllBindings()
				for _, binding := range bindings {
					pod, _ := GetPod(binding.PodID)
					if !pod.IsOccupied() {
						log.ZapLogger.Info("Unbinding model from pod", zap.String("modelUUID", binding.ModelUUID), zap.String("podID", binding.PodID))
						err := UnbindModelFromPod(binding.PodID)
						if err != nil {
							log.ZapLogger.Error("Failed to unbind model from pod", zap.Error(err))
						}
					}
				}
			}
		}()
	})
}
