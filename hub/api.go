package hub

import (
	"cotelligence-model-hub/openapi"
	"cotelligence-model-hub/version"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func registerModelHandler(c *gin.Context) {
	var body Model
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	model, err := RegisterModel(body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, model)
}

func listModelsHandler(c *gin.Context) {
	models, err := GetAllModels()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, models)
}

func startPredictionHandler(c *gin.Context) {
	taskId := GenerateTaskID()
	// Parse the incoming JSON body
	var predictionParams map[string]interface{}
	if err := c.ShouldBindJSON(&predictionParams); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON body"})
		return
	}
	err := RecordTask(Task{ID: taskId, ModelId: c.Param("modelUUID"), Body: predictionParams})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"db error": err.Error()})
	}
	// Check if the request is synchronous, default to sync
	sync := c.Query("sync") != "false"

	// If the request is synchronous, wait for the task to complete and return the result
	if sync {
		result, err := waitForTaskCompletion(taskId)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, result)
	} else {
		// If the request is asynchronous, return the task ID immediately
		c.JSON(http.StatusOK, gin.H{"taskId": taskId})
	}

}

func listPodsHandler(c *gin.Context) {
	pods, err := GetAllPods()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, pods)
}

func listBindingsHandler(c *gin.Context) {
	bindings, err := GetAllBindings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, bindings)
}

func updateModelHandler(c *gin.Context) {
	modelUUID := c.Param("modelUUID")

	var body struct {
		MaxInstanceCnt int       `json:"max_instance_cnt"`
		MinInstanceCnt int       `json:"min_instance_cnt"`
		Type           ModelType `json:"type"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := UpdateModelInstanceCnt(modelUUID, body.MaxInstanceCnt, body.MinInstanceCnt, body.Type)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func removeModelHandler(c *gin.Context) {
	modelUUID := c.Param("modelUUID")

	err := RemoveModel(modelUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "removed"})
}

func getModelHandler(c *gin.Context) {
	modelUUID := c.Param("modelUUID")

	// Use the GetSampleIO function to get the sample input and output
	reqExample, resExample, err := openapi.GetSampleIO(modelUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Unmarshal the input and output strings into JSON objects
	var inputObj, outputObj map[string]interface{}
	err = json.Unmarshal([]byte(reqExample), &inputObj)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to unmarshal input: %v", err)})
		return
	}
	err = json.Unmarshal([]byte(resExample), &outputObj)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to unmarshal output: %v", err)})
		return
	}

	// Return the sample input and output as JSON
	c.JSON(http.StatusOK, gin.H{
		"modelUUID": modelUUID,
		"input":     inputObj,
		"output":    outputObj,
	})
}

func GetTaskHandler(c *gin.Context) {
	// Get the task ID from the URL
	taskID := c.Param("taskId")
	// Get the task details from the database
	task, err := GetTask(taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Return the task details as JSON
	c.JSON(http.StatusOK, task)
}

func SetupRouter() *gin.Engine {
	router := gin.Default()

	router.POST("/register-model", registerModelHandler)
	router.POST("/prediction/:modelUUID", startPredictionHandler)
	router.GET("/models", listModelsHandler)
	router.GET("/model/:modelUUID", getModelHandler)
	router.PUT("/model/:modelUUID", updateModelHandler)
	router.DELETE("/model/:modelUUID", removeModelHandler)
	router.GET("/pods", listPodsHandler)
	router.GET("/bindings", listBindingsHandler)
	router.GET("/health",
		func(c *gin.Context) {
			c.JSON(http.StatusOK, version.GetAppInfo())
		})
	router.POST("/webhook/:taskId", WebhookHandler)
	router.GET("/sse/:taskId", SSEHandler)
	router.GET("/ws/:taskId", WebSocketHandler)
	router.GET("/task/:taskId", GetTaskHandler)

	return router
}
