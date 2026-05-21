package profiles

import (
	"testing"
	"time"
)

func TestHardLimits_local(t *testing.T) {
	p := Profile{ChainType: ChainTypeLocal}
	hl := p.HardLimits()
	if hl.MaxSpendLimit != "100000000ugnot" {
		t.Errorf("local MaxSpendLimit: got %q", hl.MaxSpendLimit)
	}
	if hl.MaxExpiresIn != 30*24*time.Hour {
		t.Errorf("local MaxExpiresIn: got %v", hl.MaxExpiresIn)
	}
	if hl.MaxAllowPathsCount != 20 {
		t.Errorf("local MaxAllowPathsCount: got %d", hl.MaxAllowPathsCount)
	}
}

func TestHardLimits_testnet(t *testing.T) {
	p := Profile{ChainType: ChainTypeTestnet}
	hl := p.HardLimits()
	if hl.MaxSpendLimit != "10000000ugnot" {
		t.Errorf("testnet MaxSpendLimit: got %q", hl.MaxSpendLimit)
	}
	if hl.MaxExpiresIn != 7*24*time.Hour {
		t.Errorf("testnet MaxExpiresIn: got %v", hl.MaxExpiresIn)
	}
	if hl.MaxAllowPathsCount != 10 {
		t.Errorf("testnet MaxAllowPathsCount: got %d", hl.MaxAllowPathsCount)
	}
}

func TestHardLimits_mainnet(t *testing.T) {
	p := Profile{ChainType: ChainTypeMainnet}
	hl := p.HardLimits()
	if hl.MaxSpendLimit != "1000ugnot" {
		t.Errorf("mainnet MaxSpendLimit: got %q", hl.MaxSpendLimit)
	}
	if hl.MaxExpiresIn != time.Hour {
		t.Errorf("mainnet MaxExpiresIn: got %v", hl.MaxExpiresIn)
	}
	if hl.MaxAllowPathsCount != 3 {
		t.Errorf("mainnet MaxAllowPathsCount: got %d", hl.MaxAllowPathsCount)
	}
}

func TestHardLimits_unknown(t *testing.T) {
	p := Profile{ChainType: "foobar"}
	hl := p.HardLimits()
	if hl.MaxSpendLimit != "10000000ugnot" {
		t.Errorf("unknown chain-type should use testnet MaxSpendLimit: got %q", hl.MaxSpendLimit)
	}
	if hl.MaxExpiresIn != 7*24*time.Hour {
		t.Errorf("unknown chain-type should use testnet MaxExpiresIn: got %v", hl.MaxExpiresIn)
	}
	if hl.MaxAllowPathsCount != 10 {
		t.Errorf("unknown chain-type should use testnet MaxAllowPathsCount: got %d", hl.MaxAllowPathsCount)
	}
}

func TestHardLimits_bypassReturnsUnlimited(t *testing.T) {
	p := Profile{ChainType: ChainTypeTestnet, BypassHardLimits: true}
	hl := p.HardLimits()
	if hl.MaxSpendLimit != "" {
		t.Errorf("bypass: MaxSpendLimit should be empty (unlimited), got %q", hl.MaxSpendLimit)
	}
	if hl.MaxExpiresIn != 0 {
		t.Errorf("bypass: MaxExpiresIn should be 0 (unlimited), got %v", hl.MaxExpiresIn)
	}
	if hl.MaxAllowPathsCount != 0 {
		t.Errorf("bypass: MaxAllowPathsCount should be 0 (unlimited), got %d", hl.MaxAllowPathsCount)
	}
}
