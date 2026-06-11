module github.com/iggy/botkube

go 1.26.4

require (
	github.com/AlecAivazis/survey/v2 v2.3.7
	github.com/DanielTitkov/go-adaptive-cards v0.2.2
	github.com/MakeNowJust/heredoc/v2 v2.0.1
	github.com/Masterminds/semver/v3 v3.5.0
	github.com/PagerDuty/go-pagerduty v1.8.0
	github.com/alexflint/go-arg v1.6.1
	github.com/avast/retry-go/v5 v5.0.0
	github.com/aws/aws-sdk-go-v2 v1.42.0
	github.com/aws/aws-sdk-go-v2/config v1.32.23
	github.com/aws/aws-sdk-go-v2/credentials v1.19.24
	github.com/aws/aws-sdk-go-v2/service/sts v1.43.3
	github.com/briandowns/spinner v1.23.2
	github.com/bwmarrin/discordgo v0.29.0
	github.com/denisbrodbeck/machineid v1.0.1
	github.com/dustin/go-humanize v1.0.1
	github.com/fatih/color v1.19.0
	github.com/go-playground/locales v0.14.1
	github.com/go-playground/universal-translator v0.18.1
	github.com/go-playground/validator/v10 v10.30.3
	github.com/go-task/slim-sprig v2.20.0+incompatible
	github.com/go-viper/mapstructure/v2 v2.5.0
	github.com/google/go-github/v88 v88.0.0
	github.com/google/uuid v1.6.0
	github.com/gookit/color v1.6.1
	github.com/gorilla/mux v1.8.1
	github.com/hashicorp/go-getter/v2 v2.2.3
	github.com/hashicorp/go-hclog v1.6.3
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hashicorp/go-plugin v1.8.0
	github.com/hashicorp/go-version v1.9.0
	github.com/hasura/go-graphql-client v0.16.0
	github.com/hokaccha/go-prettyjson v0.0.0-20211117102719-0474bc63780f
	github.com/infracloudio/msbotbuilder-go v0.2.6-0.20231130085215-84d2040b3577
	github.com/knadh/koanf/parsers/yaml v1.1.0
	github.com/knadh/koanf/providers/env/v2 v2.0.0
	github.com/knadh/koanf/providers/rawbytes v1.0.0
	github.com/knadh/koanf/providers/structs v1.0.0
	github.com/knadh/koanf/v2 v2.3.5
	github.com/mattermost/mattermost/server/public v0.4.2
	github.com/mattn/go-isatty v0.0.22
	github.com/mattn/go-shellwords v1.0.13
	github.com/mitchellh/mapstructure v1.5.1-0.20231216201459-8508981c8b6c
	github.com/morikuni/aec v1.1.0
	github.com/muesli/reflow v0.3.0
	github.com/olivere/elastic/v7 v7.0.32
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.23.2
	github.com/prometheus/common v0.68.1
	github.com/r3labs/diff/v3 v3.0.2
	github.com/sanity-io/litter v1.5.8
	github.com/segmentio/analytics-go v3.1.0+incompatible
	github.com/sirupsen/logrus v1.9.4
	github.com/slack-go/slack v0.25.0
	github.com/sourcegraph/conc v0.3.1-0.20240121214520-5f936abd7ae8
	github.com/spf13/cobra v1.10.2
	github.com/spf13/pflag v1.0.10
	github.com/spiffe/spire v1.15.1
	github.com/stretchr/testify v1.11.1
	github.com/xeipuuv/gojsonschema v1.2.0
	github.com/xyproto/randomstring v1.2.0
	go.szostok.io/version v1.2.0
	golang.org/x/exp v0.0.0-20260603202125-055de637280b
	golang.org/x/oauth2 v0.36.0
	golang.org/x/sync v0.21.0
	golang.org/x/text v0.38.0
	google.golang.org/grpc v1.81.1
	google.golang.org/protobuf v1.36.12-0.20260120151049-f2248ac996af
	gopkg.in/yaml.v3 v3.0.1
	gotest.tools/v3 v3.5.2
	helm.sh/helm/v4 v4.2.0
	k8s.io/api v0.36.1
	k8s.io/apimachinery v0.36.1
	k8s.io/cli-runtime v0.36.1
	k8s.io/client-go v0.36.1
	k8s.io/klog/v2 v2.140.0
	k8s.io/kubectl v0.36.1
	k8s.io/utils v0.0.0-20260507154919-ff6756f316d2
	sigs.k8s.io/controller-runtime v0.24.1
	sigs.k8s.io/yaml v1.6.0
)

require (
	dario.cat/mergo v1.0.2 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20250102033503-faa5f7b0171c // indirect
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/MakeNowJust/heredoc v1.0.0 // indirect
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/Masterminds/sprig/v3 v3.3.0 // indirect
	github.com/Masterminds/squirrel v1.5.4 // indirect
	github.com/ProtonMail/go-crypto v1.4.1 // indirect
	github.com/alexflint/go-scalar v1.2.0 // indirect
	github.com/araddon/dateparse v0.0.0-20210429162001-6b43995a97de // indirect
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.29 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.29 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.29 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.4.30 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.12 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.29 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.2.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.31.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.36.6 // indirect
	github.com/aws/smithy-go v1.27.1 // indirect
	github.com/aymanbagabas/go-osc52/v2 v2.0.1 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bgentry/go-netrc v0.0.0-20140422174119-9fd32a8b3d3d // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/chai2010/gettext-go v1.0.3 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/cloudflare/circl v1.6.3 // indirect
	github.com/coder/websocket v1.8.14 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.7 // indirect
	github.com/cyphar/filepath-securejoin v0.6.1 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.4.1 // indirect
	github.com/dyatlov/go-opengraph/opengraph v0.0.0-20220524092352-606d7b1e5f8a // indirect
	github.com/dylibso/observe-sdk/go v0.0.0-20240819160327-2d926c5d788a // indirect
	github.com/emicklei/go-restful/v3 v3.13.0 // indirect
	github.com/evanphx/json-patch/v5 v5.9.11 // indirect
	github.com/exponent-io/jsonpath v0.0.0-20210407135951-1de76d718b3f // indirect
	github.com/extism/go-sdk v1.7.1 // indirect
	github.com/fatih/structs v1.1.0 // indirect
	github.com/fluxcd/cli-utils v1.2.0 // indirect
	github.com/francoispqt/gojay v1.2.13 // indirect
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.13 // indirect
	github.com/go-errors/errors v1.5.1 // indirect
	github.com/go-gorp/gorp/v3 v3.1.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-openapi/jsonpointer v0.22.5 // indirect
	github.com/go-openapi/jsonreference v0.21.5 // indirect
	github.com/go-openapi/swag v0.25.5 // indirect
	github.com/go-openapi/swag/cmdutils v0.25.5 // indirect
	github.com/go-openapi/swag/conv v0.25.5 // indirect
	github.com/go-openapi/swag/fileutils v0.25.5 // indirect
	github.com/go-openapi/swag/jsonname v0.25.5 // indirect
	github.com/go-openapi/swag/jsonutils v0.25.5 // indirect
	github.com/go-openapi/swag/loading v0.25.5 // indirect
	github.com/go-openapi/swag/mangling v0.25.5 // indirect
	github.com/go-openapi/swag/netutils v0.25.5 // indirect
	github.com/go-openapi/swag/stringutils v0.25.5 // indirect
	github.com/go-openapi/swag/typeutils v0.25.5 // indirect
	github.com/go-openapi/swag/yamlutils v0.25.5 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/goccy/go-json v0.10.6 // indirect
	github.com/goccy/go-yaml v1.19.2 // indirect
	github.com/gofrs/flock v0.13.0 // indirect
	github.com/golang-jwt/jwt/v4 v4.5.2 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/btree v1.1.3 // indirect
	github.com/google/gnostic-models v0.7.1 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/go-querystring v1.2.0 // indirect
	github.com/gorilla/websocket v1.5.4-0.20250319132907-e064f32e3674 // indirect
	github.com/gosuri/uitable v0.0.4 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-safetemp v1.0.0 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/hashicorp/yamux v0.1.2 // indirect
	github.com/huandu/xstrings v1.5.0 // indirect
	github.com/ianlancetaylor/demangle v0.0.0-20240805132620-81f5be970eca // indirect
	github.com/imdario/mergo v0.3.16 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jmoiron/sqlx v1.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51 // indirect
	github.com/klauspost/compress v1.18.5 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/lann/builder v0.0.0-20180802200727-47ae307949d0 // indirect
	github.com/lann/ps v0.0.0-20150810152359-62de8c46ede0 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/lestrrat-go/backoff/v2 v2.0.8 // indirect
	github.com/lestrrat-go/blackmagic v1.0.4 // indirect
	github.com/lestrrat-go/httpcc v1.0.1 // indirect
	github.com/lestrrat-go/iter v1.0.2 // indirect
	github.com/lestrrat-go/jwx v1.2.31 // indirect
	github.com/lestrrat-go/option v1.0.1 // indirect
	github.com/lib/pq v1.12.3 // indirect
	github.com/liggitt/tabwriter v0.0.0-20181228230101-89fcab3d43de // indirect
	github.com/lucasb-eyer/go-colorful v1.3.0 // indirect
	github.com/mailru/easyjson v0.9.1 // indirect
	github.com/mattermost/go-i18n v1.11.1-0.20211013152124-5c415071e404 // indirect
	github.com/mattermost/ldap v3.0.4+incompatible // indirect
	github.com/mattermost/logr/v2 v2.0.22 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-runewidth v0.0.20 // indirect
	github.com/mattn/go-sqlite3 v2.0.3+incompatible // indirect
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d // indirect
	github.com/miekg/dns v1.1.68 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/go-testing-interface v1.14.1 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/term v0.5.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/monochromegane/go-gitignore v0.0.0-20200626010858-205db1a8cc00 // indirect
	github.com/muesli/termenv v0.16.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/oklog/run v1.2.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/pborman/uuid v1.2.1 // indirect
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/philhofer/fwd v1.2.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/procfs v0.20.1 // indirect
	github.com/redis/go-redis/v9 v9.14.0 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/rubenv/sql-migrate v1.8.1 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.2 // indirect
	github.com/segmentio/backo-go v1.1.0 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/tetratelabs/wabin v0.0.0-20230304001439-f6f874872834 // indirect
	github.com/tetratelabs/wazero v1.11.0 // indirect
	github.com/tinylib/msgp v1.6.4 // indirect
	github.com/ulikunitz/xz v0.5.15 // indirect
	github.com/vmihailenco/msgpack/v5 v5.4.1 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	github.com/wiggin77/merror v1.0.5 // indirect
	github.com/wiggin77/srslog v1.0.1 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/xlab/treeprint v1.2.0 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	github.com/xtgo/uuid v0.0.0-20140804021211-a0b114877d4c // indirect
	go.opentelemetry.io/proto/otlp v1.10.0 // indirect
	go.yaml.in/yaml/v2 v2.4.4 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.52.0 // indirect
	golang.org/x/mod v0.36.0 // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/term v0.43.0 // indirect
	golang.org/x/time v0.15.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260511170946-3700d4141b60 // indirect
	gopkg.in/asn1-ber.v1 v1.0.0-20181015200546-f715ec2f112d // indirect
	gopkg.in/evanphx/json-patch.v4 v4.13.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/apiextensions-apiserver v0.36.1 // indirect
	k8s.io/apiserver v0.36.1 // indirect
	k8s.io/component-base v0.36.1 // indirect
	k8s.io/kube-openapi v0.0.0-20260317180543-43fb72c5454a // indirect
	oras.land/oras-go/v2 v2.6.0 // indirect
	sigs.k8s.io/json v0.0.0-20250730193827-2d320260d730 // indirect
	sigs.k8s.io/kustomize/api v0.21.1 // indirect
	sigs.k8s.io/kustomize/kyaml v0.21.1 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v6 v6.3.2 // indirect
)
