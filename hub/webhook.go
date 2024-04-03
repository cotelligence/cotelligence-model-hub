package hub

import (
	"cotelligence-model-hub/log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

type TaskStatus string

const (
	Processing TaskStatus = "processing"
	Succeeded  TaskStatus = "succeeded"
)

type TaskData struct {
	TaskId string     `json:"taskId"`
	Data   []string   `json:"data"`
	Status TaskStatus `json:"status"`
}

type EventData struct {
	Output []string   `json:"output"`
	Status TaskStatus `json:"status"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var taskDataBuffer = &sync.Map{} // Change this line

func WebhookHandler(c *gin.Context) {
	taskId := c.Param("taskId")

	var eventData EventData
	if err := c.ShouldBindJSON(&eventData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON body"})
		return
	}

	switch eventData.Status {
	case Processing:
		// Update the task data
		log.ZapLogger.Debug("Received output from the task", zap.String("taskId", taskId), zap.Strings("data", eventData.Output))
		taskDataBuffer.Store(taskId, TaskData{TaskId: taskId, Data: eventData.Output, Status: "processing"})
	case Succeeded:
		taskDataBuffer.Store(taskId, TaskData{TaskId: taskId, Data: eventData.Output, Status: "succeeded"})
	default:
		log.ZapLogger.Info("Invalid status in the request")
		return
	}
}

func WebSocketHandler(c *gin.Context) {
	// Upgrade the HTTP connection to a WebSocket connection
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		http.Error(c.Writer, "Could not open websocket connection", http.StatusBadRequest)
		return
	}

	// Retrieve the taskId from the URL parameters
	taskId := c.Param("taskId")
	// Create a ticker that ticks every 400 milliseconds
	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		// Retrieve the data associated with the taskId from the taskDataBuffer
		value, ok := taskDataBuffer.Load(taskId)
		if !ok {
			http.Error(c.Writer, "Task not found", http.StatusNotFound)
			conn.Close()
		}
		task := value.(TaskData)

		// Join all the strings in the data slice into a single string
		dataStr := strings.Join(task.Data, "")
		if dataStr == "" {
			continue
		}
		// remove data from the buffer
		taskDataBuffer.Store(taskId, TaskData{TaskId: taskId, Data: nil, Status: task.Status})

		// Write the data to the WebSocket connection
		err = conn.WriteMessage(websocket.TextMessage, []byte(dataStr))
		if err != nil {
			log.ZapLogger.Error("Failed to write message to websocket", zap.Error(err))
			conn.Close()
		}
		// close the connection if the task has succeeded
		if task.Status == Succeeded {
			err = conn.Close()
			// remove task from buffer
			taskDataBuffer.Delete(taskId)
			return
		}
		// Sleep for a while before checking for updates again
		time.Sleep(400 * time.Millisecond)
	}
}

func SSEHandler(c *gin.Context) {
	// Upgrade the HTTP connection to an SSE connection
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")

	// Retrieve the taskId from the URL parameters
	taskId := c.Param("taskId")
	// Create a ticker that ticks every 400 milliseconds
	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		// Retrieve the data associated with the taskId from the taskDataBuffer
		value, ok := taskDataBuffer.Load(taskId)

		if !ok {
			http.Error(c.Writer, "Task not found", http.StatusNotFound)
			return
		}
		task := value.(TaskData)

		// Join all the strings in the data slice into a single string
		dataStr := strings.Join(task.Data, "")
		if dataStr == "" {
			continue
		}
		// Write the data to the SSE connection
		c.SSEvent("message", dataStr)
		// Flush the response writer
		if flusher, ok := c.Writer.(http.Flusher); ok {
			flusher.Flush()
		} else {
			log.ZapLogger.Error("Expected http.ResponseWriter to be an http.Flusher")
			return
		}
		// remove data from the buffer
		taskDataBuffer.Store(taskId, TaskData{TaskId: taskId, Data: nil, Status: task.Status})
		// close the connection if the task has succeeded
		if task.Status == Succeeded {
			// remove task from buffer
			taskDataBuffer.Delete(taskId)
			return
		}
	}
}
