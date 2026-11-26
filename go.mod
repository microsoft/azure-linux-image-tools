module github.com/microsoft/azure-linux-image-tools

go 1.26.0

require (
	github.com/alecthomas/kong v1.16.1
	github.com/anchore/go-rpmdb v0.2.2
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2
	github.com/cavaliergopher/cpio v1.0.1
	github.com/glebarez/go-sqlite v1.23.0
	github.com/google/uuid v1.6.0
	github.com/invopop/jsonschema v0.14.0
	github.com/klauspost/compress v1.20.4
	github.com/klauspost/pgzip v1.3.0
	github.com/moby/sys/mountinfo v0.7.3
	github.com/notaryproject/notation-go v1.3.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter v0.166.0
	github.com/opencontainers/image-spec v1.1.1
	github.com/package-url/packageurl-go v0.1.7
	github.com/sirupsen/logrus v1.10.2
	github.com/spdx/tools-golang v0.5.7
	github.com/stretchr/testify v1.12.1
	go.opentelemetry.io/collector/component v1.72.0
	go.opentelemetry.io/collector/confmap v1.72.0
	go.opentelemetry.io/collector/confmap/provider/envprovider v1.72.0
	go.opentelemetry.io/collector/confmap/provider/fileprovider v1.72.0
	go.opentelemetry.io/collector/confmap/provider/yamlprovider v1.72.0
	go.opentelemetry.io/collector/consumer v1.72.0
	go.opentelemetry.io/collector/exporter v1.72.0
	go.opentelemetry.io/collector/otelcol v0.166.0
	go.opentelemetry.io/collector/pdata v1.72.0
	go.opentelemetry.io/collector/processor v1.72.0
	go.opentelemetry.io/collector/receiver v1.72.0
	go.opentelemetry.io/collector/receiver/otlpreceiver v0.166.0
	go.opentelemetry.io/collector/service v0.166.0
	go.opentelemetry.io/contrib/exporters/autoexport v0.72.0
	go.opentelemetry.io/otel v1.47.0
	go.opentelemetry.io/otel/sdk v1.47.0
	go.opentelemetry.io/otel/trace v1.47.0
	go.uber.org/zap v1.28.0
	go.yaml.in/yaml/v3 v3.0.6
	golang.org/x/sys v0.50.0
	gonum.org/v1/gonum v0.17.0
	gopkg.in/ini.v1 v1.68.0
	oras.land/oras-go/v2 v2.7.0
)

require (
	code.cloudfoundry.org/clock v0.0.0-20180518195852-02e53af36e6c // indirect
	github.com/Azure/go-ntlmssp v0.1.1 // indirect
	github.com/Microsoft/go-winio v0.6.4 // indirect
	github.com/anchore/go-struct-converter v0.1.0 // indirect
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/buger/jsonparser v1.1.2 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cenkalti/backoff/v7 v7.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/ebitengine/purego v0.11.2 // indirect
	github.com/felixge/httpsnoop v1.1.0 // indirect
	github.com/foxboron/go-tpm-keyfiles v0.0.0-20260902202739-8c9c2d1005f4 // indirect
	github.com/fxamacker/cbor/v2 v2.9.4 // indirect
	github.com/go-asn1-ber/asn1-ber v1.5.7 // indirect
	github.com/go-ldap/ldap/v3 v3.4.10 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/gofrs/uuid v4.0.0+incompatible // indirect
	github.com/golang-jwt/jwt/v4 v4.5.2 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/go-tpm v0.9.9-0.20260124013517-8f8f42cba0de // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.31.0 // indirect
	github.com/hashicorp/go-version v1.9.0 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf/maps v0.1.3 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.1 // indirect
	github.com/knadh/koanf/v2 v2.4.1 // indirect
	github.com/lufia/plan9stats v0.0.0-20260330125221-c963978e514e // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/microsoft/ApplicationInsights-Go v0.4.4 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/notaryproject/notation-core-go v1.3.0 // indirect
	github.com/notaryproject/notation-plugin-framework-go v1.0.0 // indirect
	github.com/notaryproject/tspclient-go v1.0.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.166.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent v0.166.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/pandatix/go-abnf v0.5.5 // indirect
	github.com/pb33f/ordered-map/v2 v2.3.1 // indirect
	github.com/pierrec/lz4/v4 v4.1.33 // indirect
	github.com/power-devops/perfstat v0.0.0-20260805114148-88456608a4f6 // indirect
	github.com/prometheus/client_golang v1.24.1 // indirect
	github.com/prometheus/client_model v0.6.3 // indirect
	github.com/prometheus/common v0.74.0 // indirect
	github.com/prometheus/otlptranslator v1.0.0 // indirect
	github.com/prometheus/procfs v0.22.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/rs/cors v1.11.1 // indirect
	github.com/shirou/gopsutil/v4 v4.26.10 // indirect
	github.com/spdx/gordf v0.0.0-20201111095634-7098f93598fb // indirect
	github.com/spf13/cobra v1.10.2 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/stretchr/objx v0.5.3 // indirect
	github.com/tklauser/go-sysconf v0.4.0 // indirect
	github.com/tklauser/numcpus v0.12.0 // indirect
	github.com/veraison/go-cose v1.3.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/client v1.72.0 // indirect
	go.opentelemetry.io/collector/component/componentstatus v0.166.0 // indirect
	go.opentelemetry.io/collector/component/componenttest v0.166.0 // indirect
	go.opentelemetry.io/collector/config/configauth v1.72.0 // indirect
	go.opentelemetry.io/collector/config/configcompression v1.72.0 // indirect
	go.opentelemetry.io/collector/config/configgrpc v1.72.0 // indirect
	go.opentelemetry.io/collector/config/confighttp v1.72.0 // indirect
	go.opentelemetry.io/collector/config/configmiddleware v1.72.0 // indirect
	go.opentelemetry.io/collector/config/confignet v1.72.0 // indirect
	go.opentelemetry.io/collector/config/configopaque v1.72.0 // indirect
	go.opentelemetry.io/collector/config/configoptional v1.72.0 // indirect
	go.opentelemetry.io/collector/config/configretry v1.72.0 // indirect
	go.opentelemetry.io/collector/config/configstorage v0.166.0 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.166.0 // indirect
	go.opentelemetry.io/collector/config/configtls v1.72.0 // indirect
	go.opentelemetry.io/collector/confmap/xconfmap v0.166.0 // indirect
	go.opentelemetry.io/collector/connector v0.166.0 // indirect
	go.opentelemetry.io/collector/connector/connectortest v0.166.0 // indirect
	go.opentelemetry.io/collector/connector/xconnector v0.166.0 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.166.0 // indirect
	go.opentelemetry.io/collector/consumer/consumertest v0.166.0 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.166.0 // indirect
	go.opentelemetry.io/collector/exporter/exporterhelper v0.166.0 // indirect
	go.opentelemetry.io/collector/exporter/exportertest v0.166.0 // indirect
	go.opentelemetry.io/collector/exporter/xexporter v0.166.0 // indirect
	go.opentelemetry.io/collector/extension v1.72.0 // indirect
	go.opentelemetry.io/collector/extension/extensionauth v1.72.0 // indirect
	go.opentelemetry.io/collector/extension/extensioncapabilities v0.166.0 // indirect
	go.opentelemetry.io/collector/extension/extensionmiddleware v0.166.0 // indirect
	go.opentelemetry.io/collector/extension/extensiontest v0.166.0 // indirect
	go.opentelemetry.io/collector/extension/xextension v0.166.0 // indirect
	go.opentelemetry.io/collector/featuregate v1.72.0 // indirect
	go.opentelemetry.io/collector/internal/componentalias v0.166.0 // indirect
	go.opentelemetry.io/collector/internal/fanoutconsumer v0.166.0 // indirect
	go.opentelemetry.io/collector/internal/sharedcomponent v0.166.0 // indirect
	go.opentelemetry.io/collector/internal/statusutil v0.166.0 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.166.0 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.166.0 // indirect
	go.opentelemetry.io/collector/pdata/testdata v0.166.0 // indirect
	go.opentelemetry.io/collector/pdata/xpdata v0.166.0 // indirect
	go.opentelemetry.io/collector/pipeline v1.72.0 // indirect
	go.opentelemetry.io/collector/pipeline/xpipeline v0.166.0 // indirect
	go.opentelemetry.io/collector/processor/processortest v0.166.0 // indirect
	go.opentelemetry.io/collector/processor/xprocessor v0.166.0 // indirect
	go.opentelemetry.io/collector/receiver/receiverhelper v0.166.0 // indirect
	go.opentelemetry.io/collector/receiver/receiverhelper/xreceiverhelper v0.166.0 // indirect
	go.opentelemetry.io/collector/receiver/receivertest v0.166.0 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.166.0 // indirect
	go.opentelemetry.io/collector/service/hostcapabilities v0.166.0 // indirect
	go.opentelemetry.io/contrib/bridges/prometheus v0.72.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.72.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.72.0 // indirect
	go.opentelemetry.io/contrib/otelconf v0.27.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc v0.23.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp v0.23.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.47.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.47.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.47.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.47.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.47.0 // indirect
	go.opentelemetry.io/otel/exporters/prometheus v0.69.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutlog v0.23.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutmetric v1.47.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.47.0 // indirect
	go.opentelemetry.io/otel/log v1.47.0 // indirect
	go.opentelemetry.io/otel/metric v1.47.0 // indirect
	go.opentelemetry.io/otel/sdk/log v1.47.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.47.0 // indirect
	go.opentelemetry.io/proto/otlp v1.11.1 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v4 v4.0.0-rc.2 // indirect
	golang.org/x/crypto v0.59.0 // indirect
	golang.org/x/exp v0.0.0-20260908205506-85c1c2202aba // indirect
	golang.org/x/mod v0.42.0 // indirect
	golang.org/x/net v0.63.0 // indirect
	golang.org/x/sync v0.25.0 // indirect
	golang.org/x/text v0.44.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260928230214-8a89bd6388cc // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260928230214-8a89bd6388cc // indirect
	google.golang.org/grpc v1.85.0 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
	modernc.org/libc v1.74.1 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
	modernc.org/sqlite v1.55.0 // indirect
)
