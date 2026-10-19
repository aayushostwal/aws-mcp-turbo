package turbo

import "github.com/aws/aws-sdk-go-v2/aws"

// NewAWSEngine serves every configured AWS service from one shared credential
// and retry configuration. Clients are created per service and region on first use.
func NewAWSEngine(cfg aws.Config) *Engine {
	e := NewEngine(Registry{})
	e.Services = NewServiceSet(cfg)
	e.defaultRegion = cfg.Region
	return e
}
