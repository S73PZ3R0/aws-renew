package updater

import (
	"context"
	"testing"

	awssvc "github.com/S73PZ3R0/aws-renew/internal/aws"
)

func makeInstance(sgIDs ...string) awssvc.Instance {
	sgs := make([]awssvc.SecurityGroupRef, len(sgIDs))
	for i, id := range sgIDs {
		sgs[i] = awssvc.SecurityGroupRef{GroupID: id}
	}
	return awssvc.Instance{
		InstanceID:     "i-123",
		SecurityGroups: sgs,
	}
}

func TestUpdaterMatchingRules(t *testing.T) {
	inst := makeInstance("sg-1")
	// nil client is fine because dry-run=true makes no AWS calls
	u, err := New(inst, []int{22, 443}, []string{"1.1.1.1"}, nil, true, false, "")
	if err != nil {
		t.Fatal(err)
	}

	rules := []awssvc.SecurityGroupRule{
		{GroupID: "sg-1", IsEgress: false, IpProtocol: "tcp", FromPort: 22, ToPort: 22, CidrIpv4: "1.1.1.1/32", SecurityGroupRuleID: "sgr-1", Description: DefaultManagedDescription},
		{GroupID: "sg-1", IsEgress: false, IpProtocol: "tcp", FromPort: 443, ToPort: 443, CidrIpv4: "1.1.1.1/32", SecurityGroupRuleID: "sgr-2", Description: DefaultManagedDescription},
	}

	res, err := u.Update(context.Background(), rules)
	if err != nil {
		t.Fatal(err)
	}
	if res.Skipped != 2 {
		t.Errorf("expected 2 skipped, got %d", res.Skipped)
	}
	if res.Updated != 0 {
		t.Errorf("expected 0 updated, got %d", res.Updated)
	}
}

func TestUpdaterStaleRuleUpdated(t *testing.T) {
	inst := makeInstance("sg-1")
	u, err := New(inst, []int{22}, []string{"2.2.2.2"}, nil, true, false, "")
	if err != nil {
		t.Fatal(err)
	}

	rules := []awssvc.SecurityGroupRule{
		{GroupID: "sg-1", IsEgress: false, IpProtocol: "tcp", FromPort: 22, ToPort: 22, CidrIpv4: "1.1.1.1/32", SecurityGroupRuleID: "sgr-1", Description: DefaultManagedDescription},
	}

	res, err := u.Update(context.Background(), rules)
	if err != nil {
		t.Fatal(err)
	}
	// dry-run so no AWS call, stale rule counts as Updated
	if res.Updated != 1 {
		t.Errorf("expected 1 updated, got %d", res.Updated)
	}
}

func TestUpdaterNonMatchingSGIgnored(t *testing.T) {
	inst := makeInstance("sg-1")
	u, err := New(inst, []int{22}, []string{"1.1.1.1"}, nil, true, false, "")
	if err != nil {
		t.Fatal(err)
	}

	rules := []awssvc.SecurityGroupRule{
		{GroupID: "sg-2", IsEgress: false, IpProtocol: "tcp", FromPort: 22, ToPort: 22, CidrIpv4: "1.1.1.1/32", Description: DefaultManagedDescription},
	}

	res, err := u.Update(context.Background(), rules)
	if err != nil {
		t.Fatal(err)
	}
	if res.Skipped != 0 || res.Updated != 1 {
		t.Errorf("expected 1 updated (new rule needed), got skipped=%d updated=%d", res.Skipped, res.Updated)
	}
}
