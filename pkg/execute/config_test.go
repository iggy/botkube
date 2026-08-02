package execute

import (
	"context"
	"testing"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iggy/botkube/pkg/config"
	"github.com/iggy/botkube/pkg/loggerx"
)

const (
	configTestClusterName = "foo"
)

func TestConfigExecutorShowConfig(t *testing.T) {
	testCases := []struct {
		Name           string
		CmdCtx         CommandContext
		Cfg            config.Config
		ExpectedResult string
	}{
		{
			Name: "Print config",
			CmdCtx: CommandContext{
				Args:           []string{"config"},
				Conversation:   Conversation{Alias: channelAlias, ID: "conv-id"},
				Platform:       config.SocketSlackCommPlatformIntegration,
				ClusterName:    configTestClusterName,
				ExecutorFilter: newExecutorTextFilter(""),
			},
			Cfg: config.Config{
				Settings: config.Settings{
					ClusterName: configTestClusterName,
				},
			},
			ExpectedResult: heredoc.Doc(`
						actions: {}
						sources: {}
						executors: {}
						aliases: {}
						communications: {}
						analytics:
						    disable: false
						settings:
						    clusterName: foo
						    upgradeNotifier: false
						    systemConfigMap: {}
						    persistentConfig:
						        startup:
						            fileName: ""
						            configMap: {}
						        runtime:
						            fileName: ""
						            configMap: {}
						    metricsPort: ""
						    healthPort: ""
						    log:
						        level: ""
						        disableColors: false
						        formatter: ""
						    informersResyncPeriod: 0s
						    kubeconfig: ""
						    saCredentialsPathPrefix: ""
						    statusCanvas:
						        enabled: false
						        channels: []
						        title: ""
						        updateInterval: 0s
						        snapshotInterval: 0s
						        namespaces:
						            include: []
						        sections:
						            summary:
						                disabled: false
						                limit: 0
						            nodes:
						                disabled: false
						                limit: 0
						            workloads:
						                disabled: false
						                limit: 0
						            catalog:
						                disabled: false
						                limit: 0
						                labelSelector: ""
						            warnings:
						                disabled: false
						                limit: 0
						configWatcher:
						    enabled: false
						    remote:
						        pollInterval: 0s
						    inCluster:
						        informerResyncPeriod: 0s
						    deployment: {}
						plugins:
						    cacheDir: ""
						    repositories: {}
						    incomingWebhook:
						        enabled: false
						        port: 0
						        inClusterBaseURL: ""
						    restartPolicy:
						        type: ""
						        threshold: 0
						    healthCheckInterval: 0s
						`),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			e := NewConfigExecutor(loggerx.NewNoop(), tc.Cfg)
			msg, err := e.Show(context.Background(), tc.CmdCtx)
			require.NoError(t, err)
			assert.Equal(t, tc.ExpectedResult, msg.BaseBody.CodeBlock)
		})
	}
}
