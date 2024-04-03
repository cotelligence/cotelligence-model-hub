package hub

import (
	"cotelligence-model-hub/db"
	"encoding/json"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

type Task struct {
	ID       string                 `json:"id"`
	ModelId  string                 `json:"model_id"`
	Response map[string]interface{} `json:"response"`
	Body     map[string]interface{} `json:"body"`
}

func GenerateTaskID() string {
	return uuid.New().String()
}

const taskQueuePrefix = "hub:taskQueue:"
const taskPrefix = "hub:task:"

func RecordTask(task Task) error {
	client := db.GetRedisClient()

	// Serialize the Body
	body, err := json.Marshal(task.Body)
	if err != nil {
		return err
	}

	// Record the task with its details
	taskKey := taskPrefix + task.ID
	_, err = client.HSet(ctx, taskKey, "ModelId", task.ModelId, "Body", body).Result()
	if err != nil {
		return err
	}

	// Set the task to expire after 30 minutes
	_, err = client.Expire(ctx, taskKey, 3*time.Hour).Result()
	if err != nil {
		return err
	}

	// Add the task to the task queue for its model
	_, err = client.LPush(ctx, taskQueuePrefix+task.ModelId, task.ID).Result()
	if err != nil {
		return err
	}

	return nil
}

func processTask(taskID string) {
	client := db.GetRedisClient()
	// Get the task details
	taskKey := taskPrefix + taskID
	taskDetails, err := client.HGetAll(ctx, taskKey).Result()
	if err != nil {
		// the task may already have been processed/expired
		return
	}

	// Deserialize the Body
	var body map[string]interface{}
	err = json.Unmarshal([]byte(taskDetails["Body"]), &body)
	if err != nil {
		// Log the error and continue
		return
	}

	// Proxy the request to the pod
	modelId := taskDetails["ModelId"]
	response, err := ProxyRequestToPod(modelId, taskID, body)
	if err != nil {
		// Log the error and continue
		return
	}

	// Serialize the response
	serializedResponse, err := json.Marshal(response)
	if err != nil {
		// Log the error and continue
		return
	}

	// Update the task with the serialized response
	_, err = client.HSet(ctx, taskKey, "Response", string(serializedResponse)).Result()
	if err != nil {
		// Log the error and continue
		return
	}
}

func ProcessTasks() {
	client := db.GetRedisClient()

	// Fetch all models
	models, err := GetAllModels()
	if err != nil {
		// Log the error and return
		return
	}

	// Spawn a goroutine for each model
	for _, model := range models {
		go func(modelId string) {
			for {
				// Fetch a task from the task queue for the model
				result, err := client.BRPop(ctx, 0, taskQueuePrefix+modelId).Result()
				if err != nil {
					// Log the error and continue
					continue
				}

				// The task ID is the second element in the result
				taskID := result[1]

				// Process the task
				go processTask(taskID)
			}
		}(model.UUID)
	}
}

func waitForTaskCompletion(taskID string) (map[string]interface{}, error) {
	client := db.GetRedisClient()

	// Wait for the task to be processed
	taskKey := taskPrefix + taskID
	for {
		// Check if the task has a response
		response, err := client.HGet(ctx, taskKey, "Response").Result()
		if err != nil && redis.Nil != err {
			return nil, err
		}

		// If the task has a response, return it
		if response != "" {
			var jsonResponse map[string]interface{}
			err = json.Unmarshal([]byte(response), &jsonResponse)
			if err != nil {
				return nil, err
			}
			return jsonResponse, nil
		}

		// If the task does not have a response, wait for a short period before checking again
		time.Sleep(200 * time.Millisecond)
	}
}

func GetTask(taskID string) (Task, error) {
	client := db.GetRedisClient()

	// Get the task details
	taskKey := taskPrefix + taskID
	taskDetails, err := client.HGetAll(ctx, taskKey).Result()
	if err != nil {
		return Task{}, err
	}

	// Deserialize the Body
	var body map[string]interface{}
	err = json.Unmarshal([]byte(taskDetails["Body"]), &body)
	if err != nil {
		return Task{}, err
	}

	// Deserialize the Response
	var response map[string]interface{}
	if taskDetails["Response"] != "" {
		err = json.Unmarshal([]byte(taskDetails["Response"]), &response)
		if err != nil {
			return Task{}, err
		}
	}

	// Construct the Task object
	task := Task{
		ID:       taskID,
		ModelId:  taskDetails["ModelId"],
		Body:     body,
		Response: response,
	}

	return task, nil
}

func GetTaskCntByModel(modelId string) (int64, error) {
	client := db.GetRedisClient()

	// Get the length of the task queue for the model
	taskCnt, err := client.LLen(ctx, taskQueuePrefix+modelId).Result()
	if err != nil {
		return 0, err
	}

	return taskCnt, nil
}

func init() {
	ProcessTasks()
}
