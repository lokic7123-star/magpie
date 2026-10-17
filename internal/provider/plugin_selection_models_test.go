package provider

import (
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
