package chain

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	gnoclient "github.com/gnolang/gno/gno.land/pkg/gnoclient"
	"github.com/gnolang/gno/gno.land/pkg/sdk/vm"
)

// SubmissionPolicyInert is the code-submission policy under which the chain
// parks a deployed package until an approver enables it.
const SubmissionPolicyInert = string(vm.CodeSubmissionPolicyInert)

// Package statuses reported by vm/qpkgmeta_json.
const (
	PackageLive   = vm.PackageStatusLive   // deployed and callable
	PackageInert  = vm.PackageStatusInert  // submitted and parked, awaiting an approver
	PackageAbsent = vm.PackageStatusAbsent // nothing at the path
)

// maxPackageReason bounds PackageMeta.Reason in bytes. Every reason the chain
// defines is under 120 bytes; a longer one comes from a node lying about it.
const maxPackageReason = 256

// truncatedMark ends a reason cut to maxPackageReason.
const truncatedMark = "…(truncated)"

// PackageMeta is what the chain holds at a package path.
type PackageMeta struct {
	Status string `json:"status"`
	// Reason says why a parked submission is not live yet, in the chain's
	// words. Chain-authored text: wrap it before it reaches LLM-visible output.
	Reason string `json:"reason"`
	// Pending reports a parked submission: always true for PackageInert, and
	// also set on a live private realm with a redeploy parked over it.
	Pending bool `json:"pending"`
}

// Live reports whether the package at the path is callable and no newer
// submission is parked over it.
func (m PackageMeta) Live() bool { return m.Status == PackageLive && !m.Pending }

func decodePackageMeta(data []byte) (PackageMeta, error) {
	var m PackageMeta
	if err := json.Unmarshal(data, &m); err != nil {
		return PackageMeta{}, fmt.Errorf("decode package meta: %w", err)
	}
	if m.Status == "" {
		return PackageMeta{}, fmt.Errorf("decode package meta: no status in %q", data)
	}
	if len(m.Reason) > maxPackageReason {
		m.Reason = strings.ToValidUTF8(m.Reason[:maxPackageReason], "") + truncatedMark
	}
	return m, nil
}

// decodeSubmissionPolicy reads params/vm:p:code_submission_policy. A chain
// predating the policy has no such param and answers "" or null.
func decodeSubmissionPolicy(data []byte) (string, error) {
	var policy *string
	if len(data) == 0 {
		return "", nil
	}
	if err := json.Unmarshal(data, &policy); err != nil {
		return "", fmt.Errorf("decode code_submission_policy: %w", err)
	}
	if policy == nil {
		return "", nil
	}
	return *policy, nil
}

// decodeAddressList reads an address-list param. Unset answers "" or null.
func decodeAddressList(data []byte) ([]string, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var addrs []string
	if err := json.Unmarshal(data, &addrs); err != nil {
		return nil, fmt.Errorf("decode address list: %w", err)
	}
	return addrs, nil
}

// SubmissionPolicy returns the chain's code-submission policy, "" when the
// chain predates the param. Backed by params/vm:p:code_submission_policy.
func (r *Real) SubmissionPolicy(_ context.Context) (string, error) {
	qres, err := r.cli.Query(gnoclient.QueryCfg{Path: "params/vm:p:code_submission_policy"})
	if err != nil {
		return "", fmt.Errorf("params/vm:p:code_submission_policy: %w", err)
	}
	return decodeSubmissionPolicy(qres.Response.Data)
}

// RunSubmitters returns the addresses allowed to send MsgRun; empty means
// anyone may. Backed by params/vm:p:run_submitters.
func (r *Real) RunSubmitters(_ context.Context) ([]string, error) {
	qres, err := r.cli.Query(gnoclient.QueryCfg{Path: "params/vm:p:run_submitters"})
	if err != nil {
		return nil, fmt.Errorf("params/vm:p:run_submitters: %w", err)
	}
	return decodeAddressList(qres.Response.Data)
}

// PackageMeta reports whether pkgPath is live, parked or absent — the one
// query that tells a parked package from a path never submitted. Backed by
// vm/qpkgmeta_json.
func (r *Real) PackageMeta(_ context.Context, pkgPath string) (PackageMeta, error) {
	qres, err := r.cli.Query(gnoclient.QueryCfg{Path: "vm/qpkgmeta_json", Data: []byte(pkgPath)})
	if err != nil {
		return PackageMeta{}, fmt.Errorf("vm/qpkgmeta_json: %w", err)
	}
	return decodePackageMeta(qres.Response.Data)
}
