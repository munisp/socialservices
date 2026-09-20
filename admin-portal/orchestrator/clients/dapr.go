package clients

import (
	"context"
	"encoding/json"
	"fmt"

	dapr "github.com/dapr/go-sdk/client"
)

type DaprClient struct {
	client dapr.Client
}

func NewDaprClient() (*DaprClient, error) {
	client, err := dapr.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create Dapr client: %w", err)
	}

	return &DaprClient{client: client}, nil
}

func (d *DaprClient) InvokeService(ctx context.Context, appID, method string, data interface{}) ([]byte, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal service request: %w", err)
	}

	content := &dapr.DataContent{
		Data:        payload,
		ContentType: "application/json",
	}

	resp, err := d.client.InvokeMethodWithContent(ctx, appID, method, "post", content)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke service: %w", err)
	}

	return resp, nil
}

func (d *DaprClient) PublishEvent(ctx context.Context, pubsubName, topic string, data interface{}) error {
	if err := d.client.PublishEvent(ctx, pubsubName, topic, data); err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}
	return nil
}

func (d *DaprClient) SaveState(ctx context.Context, storeName, key string, value interface{}) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal state value: %w", err)
	}
	if err := d.client.SaveState(ctx, storeName, key, payload, nil); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}
	return nil
}

func (d *DaprClient) GetState(ctx context.Context, storeName, key string) (*dapr.StateItem, error) {
	item, err := d.client.GetState(ctx, storeName, key, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get state: %w", err)
	}
	return item, nil
}

func (d *DaprClient) DeleteState(ctx context.Context, storeName, key string) error {
	if err := d.client.DeleteState(ctx, storeName, key, nil); err != nil {
		return fmt.Errorf("failed to delete state: %w", err)
	}
	return nil
}

func (d *DaprClient) Close() {
	d.client.Close()
}
