module github.com/esai-dev/aws-lambda-aws-to-slack

go 1.26

require (
	github.com/aws/aws-lambda-go v1.54.0
	github.com/aws/aws-sdk-go-v2 v1.46.0
	github.com/aws/aws-sdk-go-v2/config v1.32.37
	github.com/aws/aws-sdk-go-v2/service/cloudwatch v1.71.0
	github.com/aws/aws-sdk-go-v2/service/codecommit v1.36.3
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.63.4
	github.com/aws/aws-sdk-go-v2/service/kms v1.59.0
	github.com/aws/aws-sdk-go-v2/service/s3 v1.106.2
	github.com/google/go-cmp v0.7.0
	golang.org/x/sync v0.22.0
)

require (
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.15 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.19.36 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.37 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.5.2 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.8.2 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.4.38 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.9.26 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.12.15 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.37 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.19.34 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.5.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.33.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.38.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.45.6 // indirect
	github.com/aws/smithy-go v1.28.1 // indirect
)
