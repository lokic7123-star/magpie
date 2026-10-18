package provider

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/yetone/magpie/internal/plugin"
	"github.com/yetone/magpie/internal/settings"
	"github.com/yetone/magpie/internal/testenv"
)

// pluginCheckinSandbox is the fake plugin with its own check-in, signed in
// with each key; it says the file each press is written to.
func pluginCheckinSandbox(t *testing.T, keys ...string) string {
	t.Helper()
	bun := testenv.Bun(t)
	home := claudeHome(t)
	t.Setenv("MAGPIE_BUN", bun)
	presses := filepath.Join(home, "presses")
	t.Setenv("FAKE_CHECKIN", presses)
	t.Cleanup(plugin.Settle)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	abs, _ := filepath.Abs("../plugin/testdata/fake/index.js")
	if _, err := plugin.Add(ctx, abs); err != nil {
		t.Fatal(err)
	}
	if _, err := plugin.Providers(ctx); err != nil {
		t.Fatal(err)
	}
	for _, k := range keys {
		if _, err := PluginAPIKey(ctx, "fakeco", 0, nil, k); err != nil {
			t.Fatal(err)
		}
	}
	return presses
}

// pressed is the keys the plugin's check-in was pressed for, in order.
func pressed(t *testing.T, file string) []string {
	t.Helper()
	b, err := os.ReadFile(file)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Fields(string(b))
}

// A plugin's own check-in (auth.checkin; Lemon on Discord) runs as magpie's
// own do: each account once a Beijing day while it is on, a failure asked
// again after wbCheckinRetry, a captcha left for the user (never retried
// that day), and what came of each kept, on its card and in Settings.
func TestPluginCheckinRunsAsTheBuiltinsDo(t *testing.T) {
	keys := []string{"ck-claim", "ck-done", "ck-captcha", "ck-throw", "ck-odd"}
	presses := pluginCheckinSandbox(t, keys...)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	ps := pluginCheckinProviders()
	if len(ps) != 1 || ps[0].ID != "fakeco" || ps[0].Name != "FakeCo" || len(ps[0].accounts) != len(keys) {
		t.Fatalf("the plugins that check in: %+v", ps)
	}
	if !HasPluginCheckin() {
		t.Fatal("HasPluginCheckin is false")
	}

	now := time.Date(2026, 10, 6, 10, 0, 0, 0, beijing)
	loop := pluginCheckiner{now: func() time.Time { return now }, only: func(id string) bool { return PluginCheckinOn(settings.Load(), id) }}

	// off (as it is by default): nothing pressed
	if rs := loop.checkinNow(ctx, false); len(rs) != 0 || len(pressed(t, presses)) != 0 {
		t.Fatalf("while off: %+v, pressed %q", rs, pressed(t, presses))
	}
	s := settings.Load()
	s.PluginCheckins = map[string]bool{"fakeco": true}
	if err := settings.Save(s); err != nil {
		t.Fatal(err)
	}

	rs := loop.checkinNow(ctx, false)
	if len(rs) != len(keys) {
		t.Fatalf("on: %d answers, want %d: %+v", len(rs), len(keys), rs)
	}
	// by what each answered: one account per key
	got := map[string]WorkBuddyCheckin{}
	for _, r := range rs {
		if r.By != "plugin:fakeco" || r.Vendor != "FakeCo" || !r.Asked || r.Day != "2026-10-06" {
			t.Fatalf("an answer: %+v", r)
		}
		k := r.Outcome
		if k == CheckinFailed {
			k += ":" + r.Msg
		}
		got[k] = r
	}
	if r := got[CheckinClaimed]; r.Credit != 50 || r.Streak != 3 {
		t.Fatalf("ck-claim: %+v (%+v)", r, rs)
	}
	if r := got[CheckinDone]; r.Credit != 50 {
		t.Fatalf("ck-done: %+v (%+v)", r, rs)
	}
	if r := got[CheckinCaptcha]; r.Msg != "slide the puzzle" {
		t.Fatalf("ck-captcha: %+v (%+v)", r, rs)
	}
	failed := 0
	for k := range got {
		if strings.Contains(k, "check-in is down") || strings.Contains(k, "no outcome") {
			failed++
		}
	}
	if failed != 2 {
		t.Fatalf("the failures (ck-throw, ck-odd): %+v", rs)
	}
	if p := pressed(t, presses); len(p) != len(keys) {
		t.Fatalf("pressed %q", p)
	}

	// a minute on: nothing asked again, each answered as kept
	now = now.Add(time.Minute)
	if rs := loop.checkinNow(ctx, false); len(rs) != len(keys) || len(pressed(t, presses)) != len(keys) {
		t.Fatalf("a minute on: %+v, pressed %q", rs, pressed(t, presses))
	}
	// past wbCheckinRetry: the failures only, not the captcha
	now = now.Add(wbCheckinRetry)
	loop.checkinNow(ctx, false)
	if p := pressed(t, presses)[len(keys):]; strings.Join(slices.Sorted(slices.Values(p)), ",") != "ck-odd,ck-throw" {
		t.Fatalf("past the retry, pressed again %q", p)
	}
	// the next Beijing day: each again
	now = time.Date(2026, 10, 7, 0, 5, 0, 0, beijing)
	loop.checkinNow(ctx, false)
	if p := pressed(t, presses); len(p) != 2*len(keys)+2 {
		t.Fatalf("the next day, pressed %q", p)
	}

	// kept, for Settings
	pcs := PluginCheckins()
	if len(pcs) != 1 || pcs[0].ID != "fakeco" || !pcs[0].On || len(pcs[0].Checkins) != len(keys) {
		t.Fatalf("PluginCheckins = %+v", pcs)
	}
	// and on each account's card
	var qs []SubscriptionQuota
	for _, a := range ps[0].accounts {
		qs = append(qs, SubscriptionQuota{Provider: PluginID("fakeco"), User: a.User})
	}
	marked := WithCheckins(qs)
	for i, q := range marked {
		if !q.Checkins || q.CheckinBy != "plugin:fakeco" || q.Checkin == nil || q.Checkin.Day != "2026-10-07" {
			t.Fatalf("card %d: %+v %+v", i, q, q.Checkin)
		}
	}
	if qs[0].Checkins {
		t.Fatal("WithCheckins changed the cards it was given")
	}
	// a press now (Check in now, the CLI) asks the failure again at once,
	// whatever the switch; another provider's press asks none of these
	before := len(pressed(t, presses))
	if rs := CheckInPlugins(ctx, "someone-else"); len(rs) != 0 || len(pressed(t, presses)) != before {
		t.Fatalf("another provider's press: %+v", rs)
	}
}

// A plugin that checks in itself takes over from magpie's own check-in
// through its fetch, so an account isn't checked in twice; and its switch
// follows the vendor's, which it took over, until set on its own.
func TestPluginCheckinTakesOverTheVendors(t *testing.T) {
	bun := testenv.Bun(t)
	claudeHome(t)
	t.Setenv("MAGPIE_BUN", bun)
	t.Setenv("FAKE_ID", "qoder")
	t.Cleanup(plugin.Settle)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	abs, _ := filepath.Abs("../plugin/testdata/fake/index.js")
	if _, err := plugin.Add(ctx, abs); err != nil {
		t.Fatal(err)
	}
	if _, err := plugin.Providers(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := plugin.APIKey(ctx, "qoder", 0, nil, "ck-claim", plugin.NewAccount); err != nil {
		t.Fatal(err)
	}
	if _, err := plugin.Providers(ctx); err != nil {
		t.Fatal(err)
	}
	// a plugin without auth.checkin: magpie claims Qoder's credits itself
	if n := len(qoderCheckinAccounts()); n != 1 || HasPluginCheckin() {
		t.Fatalf("without the hook: %d Qoder accounts, plugins that check in %v", n, HasPluginCheckin())
	}
	// with it, the plugin does, and magpie's own lists none
	t.Setenv("FAKE_CHECKIN", filepath.Join(t.TempDir(), "presses"))
	plugin.Restart()
	if _, err := plugin.Providers(ctx); err != nil {
		t.Fatal(err)
	}
	if n := len(qoderCheckinAccounts()); n != 0 || !HasPluginCheckin() {
		t.Fatalf("with the hook: %d Qoder accounts, plugins that check in %v", n, HasPluginCheckin())
	}
	for _, c := range []struct {
		s    settings.Settings
		id   string
		want bool
	}{
		{settings.Settings{QoderCheckin: true}, "qoder", true},
		{settings.Settings{QoderCheckin: true}, QoderCNID, true},
		{settings.Settings{QoderCheckin: true, PluginCheckins: map[string]bool{"qoder": false}}, "qoder", false},
		{settings.Settings{WorkBuddyCheckin: true}, "workbuddy", true},
		{settings.Settings{TraeCheckin: true}, TraeCNID, true},
		{settings.Settings{MiniMaxCheckin: true}, MiniMaxCodeGlobalID, true},
		{settings.Settings{QoderCheckin: true}, "fakeco", false},
		{settings.Settings{PluginCheckins: map[string]bool{"fakeco": true}}, "fakeco", true},
	} {
		if got := PluginCheckinOn(c.s, c.id); got != c.want {
			t.Errorf("PluginCheckinOn(%+v, %s) = %v, want %v", c.s, c.id, got, c.want)
		}
	}
}

func TestPluginQoderPressAndIntervalRecheck(t *testing.T) {
	for _, id := range []string{"qoder", QoderCNID} {
		t.Run(id, func(t *testing.T) {
			claudeHome(t)
			t.Cleanup(plugin.Settle)
			now := time.Date(2026, 10, 17, 4, 0, 0, 0, time.UTC)
			calls := 0
			outcome := CheckinInactive
			p := pluginCheckinProvider{ID: id, accounts: []pluginCheckinAcct{{User: "synthetic", On: true, key: "qoder|synthetic", card: "qoder", do: func(context.Context) WorkBuddyCheckin {
				calls++
				return WorkBuddyCheckin{Outcome: outcome}
			}}}}
			check := func() {
				t.Helper()
				p.checkiner(func() time.Time { return now }).checkinNow(context.Background(), false)
			}
			check()
			p.checkiner(func() time.Time { return now }).checkinNow(context.Background(), true)
			if calls != 2 {
				t.Fatalf("manual Qoder check must ask now instead of reusing inactive cache; calls=%d", calls)
			}
			now = now.Add(31 * time.Minute)
			outcome = CheckinDone
			check()
			if calls != 3 {
				t.Fatalf("inactive Qoder must recheck after 30 minutes; calls=%d", calls)
			}
			now = now.Add(6 * time.Minute)
			check()
			if calls != 3 {
				t.Fatalf("current completion was rechecked too soon; calls=%d", calls)
			}
			now = now.Add(25 * time.Minute)
			check()
			if calls != 4 {
				t.Fatalf("completed result must refresh after 30 minutes; calls=%d", calls)
			}

		})
	}
}

func TestPluginOtherProviderKeepsDailyCache(t *testing.T) {
	claudeHome(t)
	t.Cleanup(plugin.Settle)
	now := time.Date(2026, 10, 17, 4, 0, 0, 0, time.UTC)
	calls := 0
	p := pluginCheckinProvider{ID: "daily-fixture", accounts: []pluginCheckinAcct{{User: "synthetic", On: true, key: "daily-fixture|synthetic", card: "daily-fixture", do: func(context.Context) WorkBuddyCheckin {
		calls++
		return WorkBuddyCheckin{Outcome: CheckinInactive}
	}}}}
	p.checkiner(func() time.Time { return now }).checkinNow(context.Background(), false)
	now = now.Add(31 * time.Minute)
	p.checkiner(func() time.Time { return now }).checkinNow(context.Background(), true)
	if calls != 1 {
		t.Fatalf("ordinary daily provider must keep its settled result; calls=%d", calls)
	}
}

func TestPluginQoderCompletedCacheCanClaimOnPress(t *testing.T) {
	for _, id := range []string{"qoder", QoderCNID} {
		t.Run(id, func(t *testing.T) {
			claudeHome(t)
			t.Cleanup(plugin.Settle)
			now := time.Date(2026, 10, 18, 4, 0, 0, 0, time.UTC)
			calls := 0
			outcome := CheckinDone
			p := pluginCheckinProvider{ID: id, accounts: []pluginCheckinAcct{{User: "synthetic", On: true, key: id + "|synthetic", card: id, do: func(context.Context) WorkBuddyCheckin {
				calls++
				return WorkBuddyCheckin{Outcome: outcome}
			}}}}
			check := func(press bool, want string, wantCalls int) {
				t.Helper()
				got := p.checkiner(func() time.Time { return now }).checkinNow(context.Background(), press)
				if len(got) != 1 || got[0].Outcome != want || calls != wantCalls {
					t.Fatalf("press=%v: got=%+v calls=%d; want outcome=%s calls=%d", press, got, calls, want, wantCalls)
				}
			}
			check(false, CheckinDone, 1)
			outcome = CheckinClaimed
			check(true, CheckinClaimed, 2)
			// The automatic loop must reuse the newly claimed result, so
			// a manual refresh does not immediately cause another claim.
			check(false, CheckinClaimed, 2)
			now = now.Add(31 * time.Minute)
			outcome = CheckinDone
			check(false, CheckinDone, 3)
		})
	}
}
