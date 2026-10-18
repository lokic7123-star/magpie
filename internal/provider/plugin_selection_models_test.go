package provider

import (
	"github.com/yetone/magpie/internal/catalog"
	"github.com/yetone/magpie/internal/plugin"
	"testing"
)

func TestPluginSelectionModelsIncludesLaterAccounts(t *testing.T) {
	claudeHome(t)
	t.Setenv("MAGPIE_BUN", "")
	pp := plugin.Provider{ID: "selection-fixture", Models: []plugin.Model{
		{ID: "old-default", Name: "Old default"},
		{ID: "deepseek-v4.1-flash", Name: "DeepSeek", Free: true},
		{ID: "deepseek-v4.1-flash-sg", Name: "DeepSeek SG", Rate: 0.03},
	}, Accounts: []plugin.Account{
		{Key: "desktop", Models: []string{"old-default"}},
		{Key: "browser", Models: []string{"deepseek-v4.1-flash", "deepseek-v4.1-flash-sg"}},
	}}
	p := pluginProvider(pp, pluginLogin{acct: pp.Accounts[0]})
	for _, unlisted := range []bool{false, true} {
		p.Unlisted = unlisted
		got := p.SelectionModels()
		if len(got) != 3 || !got[1].Free || got[2].Rate != 0.03 {
			t.Fatalf("unlisted=%v selection=%+v", unlisted, got)
		}
		if got := p.Available(); len(got) != 1 || got[0].ID != "old-default" {
			t.Fatalf("first account widened: %+v", got)
		}
		if p.Account.pluginLists("deepseek-v4.1-flash") {
			t.Fatal("editor choice granted the desktop account access")
		}
	}
	p = pluginProvider(pp, pluginLogin{acct: pp.Accounts[1]})
	if !p.Account.pluginLists("deepseek-v4.1-flash") || len(p.Available()) != 2 {
		t.Fatal("browser lost its own models")
	}
}

func TestPluginSelectionBillingIsAccountScoped(t *testing.T) {
	claudeHome(t)
	t.Setenv("MAGPIE_BUN", "")
	pp := plugin.Provider{ID: "billing-selection", Models: []plugin.Model{{ID: "fast", Rate: 0}, {ID: "free", Free: false}}, Accounts: []plugin.Account{
		{Key: "desktop", Models: []string{"fast", "free"}, Billing: map[string]plugin.ModelBilling{"fast": {}, "free": {}}},
		{Key: "browser", Models: []string{"fast", "free"}, Billing: map[string]plugin.ModelBilling{"fast": {Rate: 0.34}, "free": {Free: true}}},
	}}
	for i, a := range pp.Accounts {
		p := pluginProvider(pp, pluginLogin{acct: a})
		for _, got := range [][]catalog.Model{p.SelectionModels(), p.Available()} {
			if got[0].Rate != a.Billing["fast"].Rate || got[1].Free != (i == 1) {
				t.Fatalf("account=%s models=%+v", a.Key, got)
			}
		}
	}
}

// A failed refresh can restore an older model list without a Billing field.
// Another account's successful billing must not become this account's price.
func TestPluginSelectionBillingMissingAccountData(t *testing.T) {
	for _, tc := range []struct {
		name         string
		billing      map[string]plugin.ModelBilling
		models       []string
		otherBilling map[string]plugin.ModelBilling
		free         bool
		rate         float64
	}{
		{name: "legacy cache", models: []string{"shared", "priced"}, otherBilling: map[string]plugin.ModelBilling{"shared": {Free: true}, "priced": {Rate: 0.9}}},
		{name: "empty account billing", billing: map[string]plugin.ModelBilling{}, models: []string{"shared", "priced"}, otherBilling: map[string]plugin.ModelBilling{"shared": {Free: true}, "priced": {Rate: 0.9}}},
		{name: "partial account billing", billing: map[string]plugin.ModelBilling{"priced": {Rate: 0.2}}, models: []string{"shared", "priced"}, otherBilling: map[string]plugin.ModelBilling{"shared": {Free: true}, "priced": {Rate: 0.9}}, rate: 0.2},
		{name: "unknown list", otherBilling: map[string]plugin.ModelBilling{"shared": {Free: true}, "priced": {Rate: 0.9}}},
		{name: "all legacy metadata", models: []string{"shared", "priced"}, free: true, rate: 0.9},
		{name: "known selected account", billing: map[string]plugin.ModelBilling{"shared": {Free: true}, "priced": {Rate: 0.3}}, models: []string{"shared", "priced"}, free: true, rate: 0.3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			claudeHome(t)
			t.Setenv("MAGPIE_BUN", "")
			pp := plugin.Provider{ID: "billing-cache-fixture", Models: []plugin.Model{{ID: "shared", Free: true}, {ID: "priced", Rate: 0.9, RateWas: 1.2}}, Accounts: []plugin.Account{
				{Key: "known", Models: []string{"shared", "priced"}, Billing: tc.otherBilling},
				{Key: "selected", Models: tc.models, Billing: tc.billing},
			}}
			p := pluginProvider(pp, pluginLogin{acct: pp.Accounts[1]})
			for _, got := range [][]catalog.Model{p.SelectionModels(), p.Available()} {
				if len(got) != 2 || got[0].ID != "shared" || got[1].ID != "priced" {
					t.Fatalf("billing changed the model list: %+v", got)
				}
				if got[0].Free != tc.free || got[1].Rate != tc.rate {
					t.Fatalf("account inherited another account billing: %+v", got)
				}
				if (tc.otherBilling != nil || tc.billing != nil) && got[1].RateWas != 0 {
					t.Fatalf("account inherited another account previous rate: %+v", got[1])
				}
			}
		})
	}
}
