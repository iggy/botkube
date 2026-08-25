package sink

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/iggy/botkube/internal/health"
	"github.com/iggy/botkube/pkg/config"
	"github.com/iggy/botkube/pkg/multierror"
)

const defaultHTTPCliTimeout = 30 * time.Second

// Webhook provides functionality to notify external service about new events.
type Webhook struct {
	log logrus.FieldLogger

	URL           string
	Bindings      config.SinkBindings
	status        health.PlatformStatusMsg
	failureReason health.FailureReasonMsg
	errorMsg      string
}

// WebhookPayload contains json payload to be sent to webhook url
type WebhookPayload struct {
	Source    string    `json:"source,omitempty"`
	Data      any       `json:"data,omitempty"`
	TimeStamp time.Time `json:"timeStamp"`
}

// NewWebhook creates a new Webhook instance.
func NewWebhook(log logrus.FieldLogger, commGroupIdx int, c config.Webhook) (*Webhook, error) {
	whNotifier := &Webhook{
		log:           log,
		URL:           c.URL,
		Bindings:      c.Bindings,
		status:        health.StatusUnknown,
		failureReason: "",
	}

	return whNotifier, nil
}

// SendEvent sends an event to a configured server.
func (w *Webhook) SendEvent(ctx context.Context, rawData any, sources []string) error {
	jsonPayload := &WebhookPayload{
		Source: strings.Join(sources, ","),
		Data:   rawData,
	}

	err := w.PostWebhook(ctx, jsonPayload)
	if err != nil {
		w.setFailureReason(health.FailureReasonConnectionError, fmt.Sprintf("while sending message to webhook: %s", err.Error()))
		return fmt.Errorf("while sending message to webhook: %w", err)
	}

	w.setFailureReason("", "")
	w.log.Debugf("Message successfully sent to Webhook: %+v", rawData)
	return nil
}

// PostWebhook posts webhook to listener
func (w *Webhook) PostWebhook(ctx context.Context, jsonPayload *WebhookPayload) (err error) {
	message, err := json.Marshal(jsonPayload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.URL, bytes.NewBuffer(message))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{Timeout: defaultHTTPCliTimeout}
	resp, err := client.Do(req) //nolint:gosec // URL comes from sink config, SSRF is accepted risk
	if err != nil {
		return err
	}
	defer func() {
		deferredErr := resp.Body.Close()
		if deferredErr != nil {
			err = multierror.Append(err, deferredErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Error Posting Webhook: %s", fmt.Sprint(resp.StatusCode))
	}

	return nil
}

// IntegrationName describes the notifier integration name.
func (w *Webhook) IntegrationName() config.CommPlatformIntegration {
	return config.WebhookCommPlatformIntegration
}

// Type describes the notifier type.
func (w *Webhook) Type() config.IntegrationType {
	return config.SinkIntegrationType
}

func (w *Webhook) setFailureReason(reason health.FailureReasonMsg, errorMsg string) {
	if reason == "" {
		w.status = health.StatusHealthy
	} else {
		w.status = health.StatusUnHealthy
	}
	w.failureReason = reason
	w.errorMsg = errorMsg
}

// GetStatus gets sink status
func (w *Webhook) GetStatus() health.PlatformStatus {
	return health.PlatformStatus{
		Status:   w.status,
		Restarts: "0/0",
		Reason:   w.failureReason,
		ErrorMsg: w.errorMsg,
	}
}
