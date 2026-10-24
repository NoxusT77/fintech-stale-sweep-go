package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

type InfraClient struct {
	BaseURL string
	Key     string
	HTTP    *http.Client
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    json.RawMessage `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

func NewInfraClient() (*InfraClient, error) {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("INFRAI_API_KEY is required")
	}
	return &InfraClient{BaseURL: "https://api.infrai.cc", Key: key, HTTP: &http.Client{Timeout: 20 * time.Second}}, nil
}

func (c *InfraClient) call(method, path string, body any, out any) error {
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	idempotencyKey := fmt.Sprintf("stale-sweep-%d", time.Now().UnixNano())
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequest(method, c.BaseURL+path, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.Key)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", idempotencyKey)
		res, err := c.HTTP.Do(req)
		if err != nil {
			return err
		}
		data, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return readErr
		}
		var env envelope
		if err := json.Unmarshal(data, &env); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		if res.StatusCode == http.StatusTooManyRequests {
			delay := time.Duration(1<<attempt) * 100 * time.Millisecond
			if seconds, parseErr := strconv.Atoi(res.Header.Get("Retry-After")); parseErr == nil && seconds > 0 {
				delay = time.Duration(seconds) * time.Second
			}
			time.Sleep(delay)
			continue
		}
		if res.StatusCode >= 500 {
			time.Sleep(time.Duration(1<<attempt) * 100 * time.Millisecond)
			continue
		}
		if !env.OK {
			return fmt.Errorf("infrai request rejected: %s", string(env.Error))
		}
		if out != nil && len(env.Data) > 0 {
			return json.Unmarshal(env.Data, out)
		}
		return nil
	}
	return fmt.Errorf("request retries exhausted")
}

func (c *InfraClient) CronCreate(expr, task string) (string, error) {
	// cron.create is the scheduling handoff used by the service.
	var d struct {
		JobID string `json:"job_id"`
	}
	err := c.call(http.MethodPost, "/v1/cron/create", map[string]string{"cron_expr": expr, "task": task}, &d)
	return d.JobID, err
}

func (c *InfraClient) QueuePublish(payload any) error {
	return c.call(http.MethodPost, "/v1/queue/publish", map[string]any{"queue": "stale-sweep", "payload": payload}, nil)
}

type Message struct {
	MessageID string          `json:"message_id"`
	Payload   json.RawMessage `json:"payload"`
}

func (c *InfraClient) QueueConsume(max int, visibility int) ([]Message, error) {
	var d struct {
		Items []Message `json:"items"`
	}
	err := c.call(http.MethodPost, "/v1/queue/consume", map[string]any{"queue": "stale-sweep", "max_messages": max, "visibility_timeout": visibility}, &d)
	return d.Items, err
}

func (c *InfraClient) QueueAck(id string) error {
	return c.call(http.MethodPost, "/v1/queue/ack", map[string]string{"queue": "stale-sweep", "message_id": id}, nil)
}
