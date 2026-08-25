package status

import (
	"context"

	"github.com/sirupsen/logrus"
)

type StatusReporter interface {
	ReportDeploymentConnectionInit(ctx context.Context, k8sVer string) error
	ReportDeploymentStartup(ctx context.Context) error
	ReportDeploymentShutdown(ctx context.Context) error
	ReportDeploymentFailure(ctx context.Context, errMsg string) error
	SetLogger(logger logrus.FieldLogger)
}

// GetReporter returns a StatusReporter.
//
// Deployment status used to be reported to Botkube Cloud over GraphQL. That service is gone, so
// there is nowhere left to report to and the noop implementation is the only one.
func GetReporter(_ logrus.FieldLogger) StatusReporter {
	return newNoopStatusReporter()
}
