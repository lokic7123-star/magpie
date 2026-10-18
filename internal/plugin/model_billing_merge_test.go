package plugin

import (
	"context"
	"github.com/yetone/magpie/internal/catalog"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestModelBillingSurvivesUnpricedFirstAccount(t *testing.T) {
	sandbox(t)
	cache := catalog.CachePath()
	if err := os.MkdirAll(filepath.Dir(cache), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cache, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	abs, _ := filepath.Abs("testdata/billing/index.mjs")
	if _, err := Add(ctx, abs); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"unpriced", "known"} {
		if _, err := APIKey(ctx, "billing-fixture", 0, nil, key, NewAccount); err != nil {
			t.Fatal(err)
		}
	}
	ps, err := Providers(ctx)
	if err != nil || len(ps) != 1 {
		t.Fatalf("providers=%+v err=%v", ps, err)
	}
	p := ps[0]
	if len(p.Accounts) != 2 {
		t.Fatalf("accounts=%+v", p.Accounts)
	}
	a, b := p.Accounts[0].Billing, p.Accounts[1].Billing
	if a["fast"].Rate != 0 || a["free"].Free {
		t.Fatalf("first account inherited another's price: %+v", a)
	}
	if b["fast"].Rate != 0.34 || !b["free"].Free {
		t.Fatalf("browser lost billing: %+v", b)
	}
	if a["priced"].Rate != 0.7 || b["priced"].Rate != 0.9 {
		t.Fatal("rates merged across accounts")
	}
	if a["unknown"].Free || b["unknown"].Free {
		t.Fatal("unknown became free")
	}

	if len(p.Models) != 5 || len(p.Accounts) != 2 {
		t.Fatalf("models/accounts=%+v", p)
	}
	if slices.Contains(p.Accounts[0].Models, "extra") || !slices.Contains(p.Accounts[1].Models, "extra") {
		t.Fatal("billing merge changed account entitlement")
	}
}

func TestFailedAccountBillingKeepsItsLastRead(t *testing.T) {
	last := []Provider{{ID: "billing-fixture", Spec: "fixture", Accounts: []Account{{Key: "a", Models: []string{"fast"}, Billing: map[string]ModelBilling{"fast": {Rate: 0.34}}}}}}
	now := []Provider{{ID: "billing-fixture", Spec: "fixture", Accounts: []Account{{Key: "a", FellBack: true, Models: []string{"fast"}, Billing: map[string]ModelBilling{"fast": {}}}}}}
	got := keepListed(now, last)
	if got[0].Accounts[0].Billing["fast"].Rate != 0.34 {
		t.Fatal("failed read erased known account rate")
	}
}

func TestFailedAccountFromLegacyCacheKeepsUnknownBilling(t *testing.T) {
	last := []Provider{{ID: "billing-fixture", Spec: "fixture", Accounts: []Account{{Key: "legacy", Models: []string{"shared"}}}}}
	now := []Provider{{ID: "billing-fixture", Spec: "fixture", Accounts: []Account{{Key: "legacy", FellBack: true}}}}
	got := keepListed(now, last)
	account := got[0].Accounts[0]
	if !slices.Equal(account.Models, []string{"shared"}) || account.Billing != nil {
		t.Fatalf("legacy cache should keep the list without inventing billing: %+v", account)
	}
}
