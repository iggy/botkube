package audit

import (
	"context"

	"github.com/sirupsen/logrus"
)

// AuditReporter defines interface for reporting audit events
type AuditReporter interface {
	ReportExecutorAuditEvent(ctx context.Context, e ExecutorAuditEvent) error
	ReportSourceAuditEvent(ctx context.Context, e SourceAuditEvent) error
}

// ExecutorAuditEvent contains audit event data
type ExecutorAuditEvent struct {
	CreatedAt               string
	PluginName              string
	PlatformUser            string
	BotPlatform             string
	Command                 string
	Channel                 string
	AdditionalCreateContext map[string]interface{}
}

// SourceAuditEvent contains audit event data
type SourceAuditEvent struct {
	CreatedAt  string
	PluginName string
	Event      string
	Source     SourceDetails
}

type SourceDetails struct {
	Name        string
	DisplayName string
}

// GetReporter creates new AuditReporter.
//
// Audit events used to be shipped to Botkube Cloud over GraphQL. That service is gone, so there is
// nowhere left to report to and the noop implementation is the only one.
func GetReporter(logger logrus.FieldLogger) AuditReporter {
	return newNoopAuditReporter(logger)
}
