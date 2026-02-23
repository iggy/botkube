package sink

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/olivere/elastic/v7"
	"github.com/sirupsen/logrus"

	"github.com/kubeshop/botkube/internal/health"
	botkubeconfig "github.com/kubeshop/botkube/pkg/config"
	"github.com/kubeshop/botkube/pkg/multierror"
	"github.com/kubeshop/botkube/pkg/sliceutil"
)

// awsSigningTransport is an http.RoundTripper that signs every request with
// AWS Signature Version 4 before forwarding it to the underlying transport.
type awsSigningTransport struct {
	signer      *v4.Signer
	credentials aws.CredentialsProvider
	region      string
	service     string
	next        http.RoundTripper
}

func (t *awsSigningTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request so we don't mutate the caller's copy.
	cloned := req.Clone(req.Context())

	// Read and restore the body so the signer can hash it.
	var bodyBytes []byte
	if req.Body != nil && req.Body != http.NoBody {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("while reading request body for AWS signing: %w", err)
		}
		cloned.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	// Compute SHA-256 hash of the payload.
	hash := sha256.Sum256(bodyBytes)
	payloadHash := hex.EncodeToString(hash[:])

	// Retrieve current credentials.
	creds, err := t.credentials.Retrieve(req.Context())
	if err != nil {
		return nil, fmt.Errorf("while retrieving AWS credentials for signing: %w", err)
	}

	if err := t.signer.SignHTTP(cloned.Context(), creds, cloned, payloadHash, t.service, t.region, time.Now()); err != nil {
		return nil, fmt.Errorf("while signing AWS request: %w", err)
	}

	transport := t.next
	if transport == nil {
		transport = http.DefaultTransport
	}
	return transport.RoundTrip(cloned)
}

var _ Sink = &Elasticsearch{}

const (
	// indexSuffixFormat is the date format that would be appended to the index name
	indexSuffixFormat = "2006-01-02" // YYYY-MM-DD
	// awsService for the AWS client to authenticate against
	awsService = "es"
	// AWS Role ARN from POD env variable while using IAM Role for service account
	awsRoleARNEnvName = "AWS_ROLE_ARN"
	// The token file mount path in POD env variable while using IAM Role for service account
	// #nosec G101
	awsWebIDTokenFileEnvName = "AWS_WEB_IDENTITY_TOKEN_FILE"

	elasticErrorReasonResourceAlreadyExists = "resource_already_exists_exception"
)

// Elasticsearch provides integration with the Elasticsearch solution.
type Elasticsearch struct {
	log            logrus.FieldLogger
	reporter       AnalyticsReporter
	client         *elastic.Client
	indices        map[string]botkubeconfig.ELSIndex
	clusterVersion string
	status         health.PlatformStatusMsg
	failureReason  health.FailureReasonMsg
	errorMsg       string
}

// NewElasticsearch creates a new Elasticsearch instance.
func NewElasticsearch(log logrus.FieldLogger, commGroupIdx int, c botkubeconfig.Elasticsearch, reporter AnalyticsReporter) (*Elasticsearch, error) {
	var elsClient *elastic.Client
	var err error

	var elsOpts []elastic.ClientOptionFunc
	switch c.LogLevel {
	case "info":
		elsOpts = append(elsOpts, elastic.SetInfoLog(log))
	case "error":
		elsOpts = append(elsOpts, elastic.SetInfoLog(log), elastic.SetErrorLog(log))
	case "trace":
		elsOpts = append(elsOpts, elastic.SetInfoLog(log), elastic.SetErrorLog(log), elastic.SetTraceLog(log))
	}

	if c.AWSSigning.Enabled {
		// Load the default AWS config (reads env vars, shared config, instance metadata, etc.)
		awsCfg, err := config.LoadDefaultConfig(context.Background(),
			config.WithRegion(c.AWSSigning.AWSRegion),
		)
		if err != nil {
			return nil, fmt.Errorf("while loading AWS config: %w", err)
		}

		// Use OIDC token to generate credentials if using IAM Role for Service Account (IRSA).
		awsRoleARN := os.Getenv(awsRoleARNEnvName)
		awsWebIdentityTokenFile := os.Getenv(awsWebIDTokenFileEnvName)
		var credsProvider aws.CredentialsProvider
		if awsRoleARN != "" && awsWebIdentityTokenFile != "" {
			stsClient := sts.NewFromConfig(awsCfg)
			p := stscreds.NewWebIdentityRoleProvider(stsClient, awsRoleARN, stscreds.IdentityTokenFile(awsWebIdentityTokenFile))
			credsProvider = aws.NewCredentialsCache(p)
		} else if c.AWSSigning.RoleArn != "" {
			stsClient := sts.NewFromConfig(awsCfg)
			p := stscreds.NewAssumeRoleProvider(stsClient, c.AWSSigning.RoleArn)
			credsProvider = aws.NewCredentialsCache(p)
		} else {
			// Fall back to the credential chain resolved by LoadDefaultConfig
			// (env vars → shared credentials → EC2 instance metadata, etc.).
			credsProvider = awsCfg.Credentials
		}

		transport := &awsSigningTransport{
			signer:      v4.NewSigner(),
			credentials: credsProvider,
			region:      c.AWSSigning.AWSRegion,
			service:     awsService,
		}
		awsClient := &http.Client{Transport: transport}

		elsOpts = append(elsOpts,
			elastic.SetURL(c.Server),
			elastic.SetScheme("https"),
			elastic.SetHttpClient(awsClient),
			elastic.SetSniff(false),
			elastic.SetHealthcheck(false),
			elastic.SetGzip(false),
		)
	} else {
		elsOpts = append(elsOpts,
			elastic.SetURL(c.Server),
			elastic.SetBasicAuth(c.Username, c.Password),
			elastic.SetSniff(false),
			elastic.SetHealthcheck(false),
			elastic.SetGzip(true),
		)

		if c.SkipTLSVerify {
			tr := &http.Transport{
				// #nosec G402
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
			httpClient := &http.Client{Transport: tr}
			elsOpts = append(elsOpts, elastic.SetHttpClient(httpClient))
		}
	}

	elsClient, err = elastic.NewClient(elsOpts...)
	if err != nil {
		return nil, fmt.Errorf("while creating new Elastic client: %w", err)
	}
	pong, _, err := elsClient.Ping(c.Server).Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("while pinging cluster: %w", err)
	}

	esNotifier := &Elasticsearch{
		log:            log,
		reporter:       reporter,
		client:         elsClient,
		indices:        c.Indices,
		clusterVersion: pong.Version.Number,
		status:         health.StatusUnknown,
		failureReason:  "",
	}

	err = reporter.ReportSinkEnabled(esNotifier.IntegrationName(), commGroupIdx)
	if err != nil {
		log.Errorf("report analytics error: %s", err.Error())
	}

	return esNotifier, nil
}

type mapping struct {
	Settings settings `json:"settings"`
}

type settings struct {
	Index index `json:"index"`
}

type index struct {
	Shards   int `json:"number_of_shards"`
	Replicas int `json:"number_of_replicas"`
}

func (e *Elasticsearch) flushIndex(ctx context.Context, indexCfg botkubeconfig.ELSIndex, event interface{}) error {
	// Construct the ELS Index Name with timestamp suffix
	indexName := indexCfg.Name + "-" + time.Now().Format(indexSuffixFormat)
	// Create index if not exists
	exists, err := e.client.IndexExists(indexName).Do(ctx)
	if err != nil {
		return fmt.Errorf("while getting index: %w", err)
	}
	if !exists {
		// Create a new index.
		mapping := mapping{
			Settings: settings{
				index{
					Shards:   indexCfg.Shards,
					Replicas: indexCfg.Replicas,
				},
			},
		}
		_, err := e.client.CreateIndex(indexName).BodyJson(mapping).Do(ctx)
		if err != nil && elastic.ErrorReason(err) != elasticErrorReasonResourceAlreadyExists {
			return fmt.Errorf("while creating index: %w", err)
		}
	}

	// Send event to els
	indexService := e.client.Index().Index(indexName)
	majorVersion, err := esMajorClusterVersion(e.clusterVersion)
	if err != nil {
		return fmt.Errorf("while getting cluster major version: %w", err)
	}
	if majorVersion <= 7 && indexCfg.Type != "" {
		// Only Elasticsearch <= 7.x supports Type parameter
		// nolint:staticcheck
		indexService.Type(indexCfg.Type)
	}
	_, err = indexService.BodyJson(event).Do(ctx)
	if err != nil {
		return fmt.Errorf("while posting data to ELS: %w", err)
	}
	_, err = e.client.Flush().Index(indexName).Do(ctx)
	if err != nil {
		return fmt.Errorf("while flushing data in ELS: %w", err)
	}
	e.log.Debugf("Event successfully sent to Elasticsearch index %s", indexName)
	return nil
}

// SendEvent sends an event to a configured elasticsearch server.
func (e *Elasticsearch) SendEvent(ctx context.Context, rawData any, sources []string) error {
	e.log.Debugf(">> Sending to Elasticsearch: %+v", rawData)

	errs := multierror.New()
	for _, indexCfg := range e.indices {
		if !sliceutil.Intersect(indexCfg.Bindings.Sources, sources) {
			continue
		}
		err := e.flushIndex(ctx, indexCfg, rawData)
		if err != nil {
			e.setFailureReason(health.FailureReasonConnectionError, fmt.Sprintf("while sending event to Elasticsearch index %q: %s", indexCfg.Name, err.Error()))
			errs = multierror.Append(errs, fmt.Errorf("while sending event to Elasticsearch index %q: %w", indexCfg.Name, err))
			continue
		}

		e.setFailureReason("", "")
		e.log.Debugf("Event successfully sent to Elasticsearch index %q", indexCfg.Name)
	}

	return errs.ErrorOrNil()
}

// IntegrationName describes the notifier integration name.
func (e *Elasticsearch) IntegrationName() botkubeconfig.CommPlatformIntegration {
	return botkubeconfig.ElasticsearchCommPlatformIntegration
}

// Type describes the notifier type.
func (e *Elasticsearch) Type() botkubeconfig.IntegrationType {
	return botkubeconfig.SinkIntegrationType
}

func (e *Elasticsearch) setFailureReason(reason health.FailureReasonMsg, errorMsg string) {
	if reason == "" {
		e.status = health.StatusHealthy
	} else {
		e.status = health.StatusUnHealthy
	}
	e.failureReason = reason
	e.errorMsg = errorMsg
}

// GetStatus gets sink status
func (e *Elasticsearch) GetStatus() health.PlatformStatus {
	return health.PlatformStatus{
		Status:   e.status,
		Restarts: "0/0",
		Reason:   e.failureReason,
		ErrorMsg: e.errorMsg,
	}
}

func esMajorClusterVersion(v string) (int, error) {
	versionParts := strings.Split(v, ".")
	if len(versionParts) == 1 {
		return 0, errors.New("cluster version is not valid")
	}
	majorVersion, err := strconv.Atoi(versionParts[0])
	if err != nil {
		return 0, fmt.Errorf("failed to parse cluster version: %s", versionParts[0])
	}
	return majorVersion, nil
}
