package provider

import (
	"context"
	"testing"
	"time"
)

func TestRepairQoderPluginCampaignRecheck(t *testing.T) {
	for _, id := range []string{"qoder", QoderCNID} {
		t.Run(id, func(t *testing.T) {
			claudeHome(t)
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

func TestRepairOtherPluginKeepsDailyCache(t *testing.T) {
	claudeHome(t)
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
