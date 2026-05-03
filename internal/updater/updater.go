package updater

import (
	"context"
	"fmt"
	"strings"

	awssvc "github.com/S73PZ3R0/aws-renew/internal/aws"
	"github.com/S73PZ3R0/aws-renew/internal/network"
)

const DefaultManagedDescription = "Auto-updated by aws-renew"

type UpdateResult struct {
	Updated int
	Skipped int
	Revoked int
}

type SSHRuleUpdater struct {
	instance    awssvc.Instance
	ports       []int
	sourceCIDRs []string
	sgIDs       map[string]bool
	client      *awssvc.Client
	dryRun      bool
	cleanup     bool
	description string
}

func New(instance awssvc.Instance, ports []int, sourceIPs []string,
	client *awssvc.Client, dryRun, cleanup bool, description string) (*SSHRuleUpdater, error) {

	if description == "" {
		description = DefaultManagedDescription
	}
	cidrs := make([]string, 0, len(sourceIPs))
	for _, ip := range sourceIPs {
		ip = strings.TrimSpace(ip)
		if ip == "" {
			continue
		}
		cidr, err := network.NormalizeIP(ip)
		if err != nil {
			return nil, err
		}
		cidrs = append(cidrs, cidr)
	}
	sgIDs := make(map[string]bool)
	for _, sg := range instance.SecurityGroups {
		sgIDs[sg.GroupID] = true
	}
	return &SSHRuleUpdater{
		instance:    instance,
		ports:       ports,
		sourceCIDRs: cidrs,
		sgIDs:       sgIDs,
		client:      client,
		dryRun:      dryRun,
		cleanup:     cleanup,
		description: description,
	}, nil
}

func (u *SSHRuleUpdater) Update(ctx context.Context, rules []awssvc.SecurityGroupRule) (UpdateResult, error) {
	var res UpdateResult
	managedRuleIDs := make(map[string]bool)

	for _, port := range u.ports {
		var portRules []awssvc.SecurityGroupRule
		for _, r := range rules {
			// CRITICAL: Only manage rules that have our specific description or match our target ports
			if u.sgIDs[r.GroupID] && !r.IsEgress && r.IpProtocol == "tcp" && r.FromPort == port {
				// Even if the description doesn't match, if it's the exact port we are managing,
				// we should consider it if we want to be "Zero-Trust", but to be safe and 
				// match Python behavior, we primarily target rules we marked.
				if r.Description == u.description {
					portRules = append(portRules, r)
				}
			}
		}

		validCIDRs := make(map[string]bool)
		var validRules []awssvc.SecurityGroupRule
		for _, r := range portRules {
			cidr := r.CidrIpv4
			if cidr == "" {
				cidr = r.CidrIpv6
			}
			for _, src := range u.sourceCIDRs {
				if cidr == src {
					validCIDRs[cidr] = true
					validRules = append(validRules, r)
					managedRuleIDs[r.SecurityGroupRuleID] = true
					break
				}
			}
		}

		var staleRules []awssvc.SecurityGroupRule
		for _, r := range portRules {
			cidr := r.CidrIpv4
			if cidr == "" {
				cidr = r.CidrIpv6
			}
			if !validCIDRs[cidr] {
				staleRules = append(staleRules, r)
			}
		}

		res.Skipped += len(validRules)

		for _, cidr := range u.sourceCIDRs {
			if validCIDRs[cidr] {
				continue
			}
			if len(staleRules) > 0 {
				primary := staleRules[0]
				staleRules = staleRules[1:]
				if !u.dryRun {
					toPort := primary.ToPort
					if toPort == 0 {
						toPort = port
					}
					if err := u.client.ModifyRule(ctx, primary.GroupID, primary.SecurityGroupRuleID, port, toPort, cidr, u.description); err != nil {
						return res, err
					}
				}
				managedRuleIDs[primary.SecurityGroupRuleID] = true
				res.Updated++
			} else {
				targetSG := ""
				for id := range u.sgIDs {
					targetSG = id
					break
				}
				if !u.dryRun {
					if err := u.client.AuthorizeIngress(ctx, targetSG, port, cidr, u.description); err != nil {
						return res, err
					}
				}
				res.Updated++
			}
		}

		// Revoke excess stale rules for THIS port.
		for _, r := range staleRules {
			if !u.dryRun {
				if err := u.client.RevokeIngress(ctx, r.GroupID, r.SecurityGroupRuleID); err != nil {
					return res, fmt.Errorf("revoke failed: %w", err)
				}
			}
			res.Revoked++
		}
	}

	// GLOBAL CLEANUP: If cleanup is enabled, revoke ANY rule in our SGs that has our 
	// description but wasn't touched/validated in this run (e.g. rules for old ports).
	if u.cleanup {
		for _, r := range rules {
			if u.sgIDs[r.GroupID] && r.Description == u.description && !managedRuleIDs[r.SecurityGroupRuleID] {
				// Double check it's not one of our current target ports (already handled)
				isCurrentPort := false
				for _, p := range u.ports {
					if r.FromPort == p {
						isCurrentPort = true
						break
					}
				}
				if !isCurrentPort {
					if !u.dryRun {
						if err := u.client.RevokeIngress(ctx, r.GroupID, r.SecurityGroupRuleID); err != nil {
							return res, fmt.Errorf("global cleanup revoke failed: %w", err)
						}
					}
					res.Revoked++
				}
			}
		}
	}

	return res, nil
}
