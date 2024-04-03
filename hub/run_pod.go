package hub

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/machinebox/graphql"
)

type RunPodClient struct {
	graphqlClient *graphql.Client
}

func (rpc *RunPodClient) StopPod(podID string) error {
	req := graphql.NewRequest(`
        mutation stopPod($input: PodStopInput!) {
            podStop(input: $input) {
                id
                desiredStatus
                lastStatusChange
            }
        }
    `)

	input := map[string]interface{}{
		"podId": podID,
	}

	req.Var("input", input)

	if err := rpc.graphqlClient.Run(context.Background(), req, nil); err != nil {
		return err
	}

	return nil
}

var (
	runPodAPIClient PodProviderAPI
	once            sync.Once
)

func InitRunPodAPIClient(graphqlEndpoint, apiKey string) PodProviderAPI {
	once.Do(func() {
		client := graphql.NewClient(graphqlEndpoint + "?api_key=" + apiKey)
		// Set up HTTP headers or other authentication mechanisms using apiKey
		client.Log = func(s string) { log.Println(s) } // Example logging function

		runPodAPIClient = &RunPodClient{
			graphqlClient: client,
		}
	})
	return runPodAPIClient
}
func GetRunPodAPIClient() PodProviderAPI {
	return runPodAPIClient
}

func (rpc *RunPodClient) ListPods() ([]Pod, error) {
	req := graphql.NewRequest(`
		query myPods {
			myself {
				id
				pods {
					id
					desiredStatus
					imageName
				}
			}
		}
	`)

	var respData struct {
		Myself struct {
			Id   string `json:"id"`
			Pods []struct {
				ID            string `json:"id"`
				DesiredStatus string `json:"desiredStatus"`
				Image         string `json:"imageName"`
			} `json:"pods"`
		} `json:"myself"`
	}
	if err := rpc.graphqlClient.Run(context.Background(), req, &respData); err != nil {
		return nil, err
	}

	// Convert the response to the []Pod type, setting isOccupied to false by default
	pods := make([]Pod, 0)
	for _, podData := range respData.Myself.Pods {
		pods = append(pods, Pod{
			ID:      podData.ID,
			IsPodUp: podData.DesiredStatus == "RUNNING",
			Image:   podData.Image,
		})
	}

	return pods, nil
}

func (rpc *RunPodClient) CreatePod(image string) (Pod, error) {
	req := graphql.NewRequest(`
        mutation Mutation($input: PodFindAndDeployOnDemandInput) {
            podFindAndDeployOnDemand(input: $input) {
                id
                imageName
            }
        }
    `)

	input := map[string]interface{}{
		"cloudType":         "SECURE",
		"containerDiskInGb": 20,
		"volumeInGb":        0,
		"dataCenterId":      "EU-RO-1",
		"deployCost":        0.36,
		"gpuCount":          1,
		"gpuTypeId":         "NVIDIA RTX A4500",
		"minMemoryInGb":     50,
		"minVcpuCount":      9,
		// TODO make this to config
		"networkVolumeId": "68a4pmfo29",
		"templateId":      "runpod-torch-v21",
		"startSsh":        true,
		"ports":           "5000/http,22/tcp",
		"imageName":       image,
	}

	req.Var("input", input)

	var respData struct {
		PodFindAndDeployOnDemand struct {
			ID        string `json:"id"`
			ImageName string `json:"imageName"`
		} `json:"podFindAndDeployOnDemand"`
	}

	if err := rpc.graphqlClient.Run(context.Background(), req, &respData); err != nil {
		return Pod{}, err
	}

	return Pod{
		ID:    respData.PodFindAndDeployOnDemand.ID,
		Image: respData.PodFindAndDeployOnDemand.ImageName,
	}, nil
}

func (rpc *RunPodClient) RemovePod(podID string) error {
	req := graphql.NewRequest(`
        mutation terminatePod($input: PodTerminateInput!) {
            podTerminate(input: $input)
        }
    `)

	input := map[string]interface{}{
		"podId": podID,
	}

	req.Var("input", input)

	if err := rpc.graphqlClient.Run(context.Background(), req, nil); err != nil {
		return err
	}

	return nil
}

func (rpc *RunPodClient) EditPod(podID, imageURL string) error {
	req := graphql.NewRequest(`
		mutation editPodJob($input: PodEditJobInput!) {
			podEditJob(input: $input) {
				id
				env
				port
				ports
				dockerArgs
				imageName
				containerDiskInGb
				volumeInGb
				volumeMountPath
				__typename
			}
		}
	`)

	input := map[string]interface{}{
		"podId":             podID,
		"imageName":         imageURL,
		"containerDiskInGb": 20,
		// 0 means we only support pre-mounted volume(for cache)
		"volumeInGb":      100,
		"volumeMountPath": "/workspace",
		"ports":           "5000/http,22/tcp",
	}

	req.Var("input", input)

	if err := rpc.graphqlClient.Run(context.Background(), req, nil); err != nil {
		return err
	}

	return nil
}

func (rpc *RunPodClient) WaitForAPIReady(podID string) error {
	apiURL := fmt.Sprintf("https://%s-5000.proxy.runpod.net", podID)
	healthURL := fmt.Sprintf("%s/health-check", apiURL)
	// max wait up to 5 minutes
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	b := backoff.WithContext(backoff.NewExponentialBackOff(), ctx)
	operation := func() error {
		resp, err := http.Get(healthURL)
		if err != nil {
			return err
		}
		if resp.StatusCode == http.StatusOK {
			// try to send 1 prediction to ensure it is ready
			// ref: https://github.com/replicate/cog/issues/966
			// post to /predictions with empty body
			// if the response is 409, it means the model is not ready yet
			// if the response is 200, it means the model is ready

			// reset backoff delay when the pod is healthy
			b.Reset()
			predictRes, err := http.Post(fmt.Sprintf("%s/predictions", apiURL), "application/json", nil)
			if err != nil {
				return err
			}
			if predictRes.StatusCode == http.StatusOK {
				return nil
			} else if predictRes.StatusCode == http.StatusConflict {
				return fmt.Errorf("pod %s is not ready, still setting up", podID)
			} else {
				return fmt.Errorf("pod %s is not ready", podID)
			}
		}
		return fmt.Errorf("pod %s is not ready", podID)
	}
	return backoff.Retry(operation, b)
}

func (rpc *RunPodClient) ResumePod(podID string) error {
	req := graphql.NewRequest(`
		mutation resume_pod($input: PodResumeInput!) {
			podResume(input: $input) {
				id
				gpuCount
			}
		}
	`)

	input := map[string]interface{}{
		"podId": podID,
		// TODO: support multiple GPUs on the same pod
		"gpuCount": 1,
	}

	req.Var("input", input)

	if err := rpc.graphqlClient.Run(context.Background(), req, nil); err != nil {
		if strings.Contains(err.Error(), "not enough free GPUs") {
			return &InsufficientGPUsError{Message: "Insufficient GPUs available"}
		} else {
			return err
		}
	}

	return nil
}
