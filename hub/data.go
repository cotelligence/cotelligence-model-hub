package hub

import (
	"context"
	"cotelligence-model-hub/db"
	"cotelligence-model-hub/log"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/go-redis/redis/v8"
)

type ModelType string

const (
	Text2Text ModelType = "Text2Text"
	Text2Img  ModelType = "Text2Img"
	Text2Vid  ModelType = "Text2Vid"
)

type Model struct {
	Name           string    `json:"name"`
	ImageURL       string    `json:"image_url"`
	UUID           string    `json:"uuid"`
	MinInstanceCnt int       `json:"min_instance_cnt"`
	MaxInstanceCnt int       `json:"max_instance_cnt"`
	Type           ModelType `json:"type"`
}

type Pod struct {
	ID       string    `json:"id"`
	IsPodUp  bool      `json:"is_pod_up"`
	Image    string    `json:"image"`
	LastUsed time.Time `json:"last_used"`
}

func (p *Pod) OccupiedUntil() time.Time {
	return p.LastUsed.Add(HotOccupiedMinutes * time.Minute)
}

func (p *Pod) IsOccupied() bool {
	return time.Now().Before(p.OccupiedUntil())
}

type ModelPodBinding struct {
	ModelUUID string `json:"model_uuid"`
	PodID     string `json:"pod_id"`
}

var ctx = context.Background()

const PodRedisPrefix = "cotelligence-model:pod"
const ModelPrefix = "cotelligence-model:model"
const BindingPrefix = "cotelligence-model:binding"

func AddModel(model Model) error {
	client := db.GetRedisClient()
	key := ModelPrefix + ":" + model.UUID
	// Convert the model to a map so it can be stored as a hash
	modelMap := make(map[string]interface{})
	modelMap["name"] = model.Name
	modelMap["image_url"] = model.ImageURL
	modelMap["uuid"] = model.UUID
	modelMap["min_instance_cnt"] = model.MinInstanceCnt
	modelMap["max_instance_cnt"] = model.MaxInstanceCnt
	modelMap["type"] = string(model.Type)

	return client.HSet(ctx, key, modelMap).Err()
}

func UpdateModelInstanceCnt(modelUUID string, maxInstanceCnt int, minInstanceCnt int, modelType ModelType) error {
	client := db.GetRedisClient()

	// Update the max_instance_cnt and min_instance_cnt fields for the model
	modelKey := ModelPrefix + ":" + modelUUID
	_, err := client.HSet(ctx, modelKey,
		"max_instance_cnt", maxInstanceCnt,
		"min_instance_cnt", minInstanceCnt,
		"type", string(modelType)).Result()
	return err
}

func GetModel(uuid string) (Model, bool) {
	client := db.GetRedisClient()
	key := ModelPrefix + ":" + uuid
	result, err := client.HGetAll(ctx, key).Result()
	if err != nil || len(result) == 0 {
		return Model{}, false
	}

	model := Model{
		Name:           result["name"],
		ImageURL:       result["image_url"],
		UUID:           result["uuid"],
		MinInstanceCnt: atoi(result["min_instance_cnt"]),
		MaxInstanceCnt: atoi(result["max_instance_cnt"]),
		Type:           ModelType(result["type"]),
	}

	return model, true
}

func GetAllModels() ([]Model, error) {
	client := db.GetRedisClient()
	keys, err := client.Keys(ctx, ModelPrefix+":*").Result()
	if err != nil {
		return nil, err
	}

	var models = make([]Model, 0)
	for _, key := range keys {
		result, err := client.HGetAll(ctx, key).Result()
		if err != nil {
			return nil, err
		}
		model := Model{
			Name:           result["name"],
			ImageURL:       result["image_url"],
			UUID:           result["uuid"],
			MinInstanceCnt: atoi(result["min_instance_cnt"]),
			MaxInstanceCnt: atoi(result["max_instance_cnt"]),
			Type:           ModelType(result["type"]),
		}
		models = append(models, model)
	}

	return models, nil
}

func atoi(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return i
}

func RemoveModel(uuid string) error {
	client := db.GetRedisClient()
	key := ModelPrefix + ":" + uuid
	return client.Del(ctx, key).Err()
}

func AddPod(pod Pod) error {
	client := db.GetRedisClient()
	key := PodRedisPrefix + ":" + pod.ID
	_, err := client.HSet(ctx, key, map[string]interface{}{
		"LastUsed": pod.LastUsed.Format(time.RFC3339),
		"Image":    pod.Image,
	}).Result()
	return err
}

func GetPod(id string) (Pod, error) {
	client := db.GetRedisClient()
	key := PodRedisPrefix + ":" + id
	result, err := client.HGetAll(ctx, key).Result()
	if err != nil {
		return Pod{}, err
	}

	pod := Pod{
		ID: id,
	}
	pod.LastUsed, err = time.Parse(time.RFC3339, result["LastUsed"])
	if err != nil {
		return Pod{}, err
	}

	return pod, nil
}

func ModifyPod(pod Pod) error {
	client := db.GetRedisClient()
	key := PodRedisPrefix + ":" + pod.ID
	_, err := client.HSet(ctx, key,
		"LastUsed", pod.LastUsed.Format(time.RFC3339),
		"Image", pod.Image,
		"IsPodUp", pod.IsPodUp).Result()
	return err
}

func RemovePod(id string) error {
	client := db.GetRedisClient()
	key := PodRedisPrefix + ":" + id
	_, err := client.Del(ctx, key).Result()
	return err
}

func BindModelToPod(binding ModelPodBinding) error {
	client := db.GetRedisClient()
	jsonBinding, err := json.Marshal(binding)
	if err != nil {
		return err
	}
	key := BindingPrefix + ":" + binding.PodID
	return client.Set(ctx, key, jsonBinding, 0).Err()
}

func GetModelPodBinding(podID string) (ModelPodBinding, bool) {
	client := db.GetRedisClient()
	key := BindingPrefix + ":" + podID
	val, err := client.Get(ctx, key).Result()
	if err != nil {
		return ModelPodBinding{}, false
	}
	var binding ModelPodBinding
	err = json.Unmarshal([]byte(val), &binding)
	if err != nil {
		return binding, false
	}
	return binding, true

}

// UnbindModelFromPod maybe we do not need this
func UnbindModelFromPod(podID string) error {
	client := db.GetRedisClient()
	key := BindingPrefix + ":" + podID
	return client.Del(ctx, key).Err()
}

func GetAllBindings() ([]ModelPodBinding, error) {
	client := db.GetRedisClient()
	keys, err := client.Keys(ctx, BindingPrefix+":*").Result()
	if err != nil {
		return nil, err
	}

	var bindings = make([]ModelPodBinding, 0)
	for _, key := range keys {
		val, err := client.Get(ctx, key).Result()
		if err != nil {
			return nil, err
		}
		var binding ModelPodBinding
		err = json.Unmarshal([]byte(val), &binding)
		if err != nil {
			return nil, err
		}
		bindings = append(bindings, binding)
	}

	return bindings, nil
}

func GetAllPodIds() (map[string]Pod, error) {
	allPods, err := GetAllPods()
	if err != nil {
		return nil, err
	}
	podMap := make(map[string]Pod)
	for _, pod := range allPods {
		podMap[pod.ID] = pod
	}
	return podMap, nil
}

func GetAllPods() ([]Pod, error) {
	client := db.GetRedisClient()
	keys, err := client.Keys(ctx, PodRedisPrefix+":*").Result()
	if err != nil {
		return nil, err
	}

	var pods = make([]Pod, 0)
	pipeline := client.Pipeline()

	cmds := make([]*redis.StringStringMapCmd, len(keys))
	for i, key := range keys {
		cmds[i] = pipeline.HGetAll(ctx, key)
	}

	_, err = pipeline.Exec(ctx)
	if err != nil {
		return nil, err
	}

	for _, cmd := range cmds {
		hash, err := cmd.Result()
		if err != nil {
			return nil, err
		}
		pod := Pod{
			ID:    strings.TrimPrefix(cmd.Args()[1].(string), PodRedisPrefix+":"),
			Image: hash["Image"],
		}
		// Parse LastUsed time if needed
		if LastUsed, ok := hash["LastUsed"]; ok {
			pod.LastUsed, err = time.Parse(time.RFC3339, LastUsed)
			if err != nil {
				return nil, err
			}
		}
		pods = append(pods, pod)
	}

	return pods, nil
}

func init() {
	go syncPods()
}

func syncPods() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		log.ZapLogger.Info("Starting pod synchronization")
		err := SyncPods(GetRunPodAPIClient())
		if err != nil {
			log.ZapLogger.Error("Error during pod synchronization", zap.Error(err))
			// handle the error, for example, you might want to continue to the next iteration
			continue
		}
	}
}
