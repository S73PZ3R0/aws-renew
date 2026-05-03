package aws

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"
)

// ── Error types ──────────────────────────────────────────────────────────────

type AWSAuthError struct{ msg string }

func (e *AWSAuthError) Error() string { return e.msg }

type AWSConfigError struct{ msg string }

func (e *AWSConfigError) Error() string { return e.msg }

var authCodes = map[string]bool{
	"AuthFailure": true, "InvalidClientTokenId": true,
	"SignatureDoesNotMatch": true, "ExpiredToken": true,
	"ExpiredTokenException": true, "UnauthorizedOperation": true,
	"AccessDenied": true,
}

func classifyErr(err error) error {
	if err == nil {
		return nil
	}
	var ae smithy.APIError
	if errors.As(err, &ae) {
		if authCodes[ae.ErrorCode()] {
			return &AWSAuthError{ae.Error()}
		}
		if strings.Contains(ae.ErrorMessage(), "specify a region") {
			return &AWSConfigError{ae.Error()}
		}
	}
	return err
}

// ── Client ───────────────────────────────────────────────────────────────────

type Client struct {
	ec2    *ec2.Client
	Profile string
	Region  string
}

// NewClient creates an EC2 SDK client.  If AWS_ACCESS_KEY_ID is set in the
// environment (injected by the interactive credential prompt) it is used
// directly; otherwise the default credential chain is used.
func NewClient(ctx context.Context, profile, region string) (*Client, error) {
	opts := []func(*awsconfig.LoadOptions) error{}
	if profile != "" {
		opts = append(opts, awsconfig.WithSharedConfigProfile(profile))
	}
	if region != "" {
		opts = append(opts, awsconfig.WithRegion(region))
	}
	// If the user entered credentials manually they were set as env vars;
	// prefer them explicitly so the static provider takes priority.
	if keyID := getEnv("AWS_ACCESS_KEY_ID"); keyID != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				keyID,
				getEnv("AWS_SECRET_ACCESS_KEY"),
				getEnv("AWS_SESSION_TOKEN"),
			),
		))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}
	return &Client{ec2: ec2.NewFromConfig(cfg), Profile: profile, Region: region}, nil
}

func getEnv(key string) string { return os.Getenv(key) }

// ── Domain types ─────────────────────────────────────────────────────────────

type Tag struct {
	Key   string
	Value string
}

type SecurityGroupRef struct {
	GroupID   string
	GroupName string
}

type Instance struct {
	InstanceID     string
	Tags           []Tag
	SecurityGroups []SecurityGroupRef
	Region         string
}

func (i *Instance) Name() string {
	for _, t := range i.Tags {
		if t.Key == "Name" {
			return t.Value
		}
	}
	return "N/A"
}

type SecurityGroupRule struct {
	GroupID             string
	GroupName           string
	SecurityGroupRuleID string
	IsEgress            bool
	IpProtocol          string
	FromPort            int
	ToPort              int
	CidrIpv4            string
	CidrIpv6            string
	Description         string
}

// ── EC2Service ───────────────────────────────────────────────────────────────

type EC2Service struct {
	Client *Client
}

func (s *EC2Service) ListRegions(ctx context.Context) ([]string, error) {
	out, err := s.Client.ec2.DescribeRegions(ctx, &ec2.DescribeRegionsInput{})
	if err != nil {
		return nil, classifyErr(err)
	}
	regions := make([]string, len(out.Regions))
	for i, r := range out.Regions {
		regions[i] = aws.ToString(r.RegionName)
	}
	return regions, nil
}

func (s *EC2Service) ListInstances(ctx context.Context) ([]Instance, error) {
	var all []Instance
	paginator := ec2.NewDescribeInstancesPaginator(s.Client.ec2, &ec2.DescribeInstancesInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, classifyErr(err)
		}
		for _, r := range page.Reservations {
			for _, i := range r.Instances {
				all = append(all, mapInstance(i, s.Client.Region))
			}
		}
	}
	return all, nil
}

func mapInstance(i types.Instance, region string) Instance {
	inst := Instance{
		InstanceID: aws.ToString(i.InstanceId),
		Region:     region,
	}
	for _, t := range i.Tags {
		inst.Tags = append(inst.Tags, Tag{Key: aws.ToString(t.Key), Value: aws.ToString(t.Value)})
	}
	for _, sg := range i.SecurityGroups {
		inst.SecurityGroups = append(inst.SecurityGroups, SecurityGroupRef{
			GroupID:   aws.ToString(sg.GroupId),
			GroupName: aws.ToString(sg.GroupName),
		})
	}
	return inst
}

// ── SecurityGroupService ─────────────────────────────────────────────────────

type SecurityGroupService struct {
	Client *Client
}

func (s *SecurityGroupService) ListRules(ctx context.Context) ([]SecurityGroupRule, error) {
	var all []SecurityGroupRule
	paginator := ec2.NewDescribeSecurityGroupRulesPaginator(s.Client.ec2, &ec2.DescribeSecurityGroupRulesInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, classifyErr(err)
		}
		for _, r := range page.SecurityGroupRules {
			all = append(all, mapRule(r))
		}
	}
	return all, nil
}

func (s *SecurityGroupService) ListRulesForGroups(ctx context.Context, groupIDs []string) ([]SecurityGroupRule, error) {
	if len(groupIDs) == 0 {
		return nil, nil
	}
	filters := []types.Filter{{
		Name:   aws.String("group-id"),
		Values: groupIDs,
	}}
	var all []SecurityGroupRule
	paginator := ec2.NewDescribeSecurityGroupRulesPaginator(s.Client.ec2, &ec2.DescribeSecurityGroupRulesInput{
		Filters: filters,
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, classifyErr(err)
		}
		for _, r := range page.SecurityGroupRules {
			all = append(all, mapRule(r))
		}
	}
	return all, nil
}

func mapRule(r types.SecurityGroupRule) SecurityGroupRule {
	rule := SecurityGroupRule{
		GroupID:             aws.ToString(r.GroupId),
		SecurityGroupRuleID: aws.ToString(r.SecurityGroupRuleId),
		IsEgress:            aws.ToBool(r.IsEgress),
		IpProtocol:          aws.ToString(r.IpProtocol),
		CidrIpv4:            aws.ToString(r.CidrIpv4),
		CidrIpv6:            aws.ToString(r.CidrIpv6),
		Description:         aws.ToString(r.Description),
	}
	if r.FromPort != nil {
		rule.FromPort = int(aws.ToInt32(r.FromPort))
	}
	if r.ToPort != nil {
		rule.ToPort = int(aws.ToInt32(r.ToPort))
	}
	return rule
}

// ── Mutation operations ───────────────────────────────────────────────────────

func (c *Client) ModifyRule(ctx context.Context, groupID, ruleID string, port, toPort int, cidr, description string) error {
	req := &types.SecurityGroupRuleRequest{
		IpProtocol:  aws.String("tcp"),
		FromPort:    aws.Int32(int32(port)),
		ToPort:      aws.Int32(int32(toPort)),
		Description: aws.String(description),
	}
	if strings.Contains(cidr, ":") {
		req.CidrIpv6 = aws.String(cidr)
	} else {
		req.CidrIpv4 = aws.String(cidr)
	}
	_, err := c.ec2.ModifySecurityGroupRules(ctx, &ec2.ModifySecurityGroupRulesInput{
		GroupId: aws.String(groupID),
		SecurityGroupRules: []types.SecurityGroupRuleUpdate{{
			SecurityGroupRuleId: aws.String(ruleID),
			SecurityGroupRule:   req,
		}},
	})
	return classifyErr(err)
}

func (c *Client) AuthorizeIngress(ctx context.Context, groupID string, port int, cidr, description string) error {
	perm := types.IpPermission{
		IpProtocol: aws.String("tcp"),
		FromPort:   aws.Int32(int32(port)),
		ToPort:     aws.Int32(int32(port)),
	}
	if strings.Contains(cidr, ":") {
		perm.Ipv6Ranges = []types.Ipv6Range{{CidrIpv6: aws.String(cidr), Description: aws.String(description)}}
	} else {
		perm.IpRanges = []types.IpRange{{CidrIp: aws.String(cidr), Description: aws.String(description)}}
	}
	_, err := c.ec2.AuthorizeSecurityGroupIngress(ctx, &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId:       aws.String(groupID),
		IpPermissions: []types.IpPermission{perm},
	})
	if err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) && ae.ErrorCode() == "InvalidPermission.Duplicate" {
			return nil
		}
	}
	return classifyErr(err)
}

func (c *Client) RevokeIngress(ctx context.Context, groupID, ruleID string) error {
	_, err := c.ec2.RevokeSecurityGroupIngress(ctx, &ec2.RevokeSecurityGroupIngressInput{
		GroupId:              aws.String(groupID),
		SecurityGroupRuleIds: []string{ruleID},
	})
	return classifyErr(err)
}
