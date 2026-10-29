//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPelicanAccountPoolMessagesScheduler(t *testing.T) {
	groupID := int64(4)
	repo := &mockAccountRepoForPlatform{
		accounts: []Account{
			{ID: 10, Name: "outside", Platform: PlatformAnthropic, Priority: 0, Status: StatusActive, Schedulable: true, Concurrency: 5, AccountGroups: []AccountGroup{{GroupID: groupID}}},
			{ID: 11, Name: "selected", Platform: PlatformAnthropic, Priority: 5, Status: StatusActive, Schedulable: true, Concurrency: 5, AccountGroups: []AccountGroup{{GroupID: groupID}}},
		}, accountsByID: map[int64]*Account{},
	}
	for i := range repo.accounts {
		repo.accountsByID[repo.accounts[i].ID] = &repo.accounts[i]
	}
	groups := &mockGroupRepoForGateway{groups: map[int64]*Group{groupID: {ID: groupID, Platform: PlatformAnthropic, Status: StatusActive, Hydrated: true}}}
	cfg := testConfig()
	cfg.Gateway.Scheduling.LoadBatchEnabled = true
	gateway := &GatewayService{accountRepo: repo, groupRepo: groups, cache: &mockGatewayCacheForPlatform{}, cfg: cfg, concurrencyService: NewConcurrencyService(&mockConcurrencyCache{})}
	router := &gatewayPelicanGroupRouter{gateway: gateway, concurrency: gateway.concurrencyService, slotWait: time.Millisecond}
	svc, _, _ := newPoolTestService(router, nil)
	ctx, err := svc.withAccountPool(context.Background(), groupID)
	require.NoError(t, err)
	group, model := groups.groups[groupID], "claude-3-5-sonnet-20241022"
	route, err := router.route(ctx, group, model, nil)
	require.NoError(t, err)
	require.EqualValues(t, 11, route.account.ID)
	route.release()
	_, err = router.route(ctx, group, model, map[int64]struct{}{11: {}})
	require.ErrorIs(t, err, ErrNoAvailableAccounts)
	route, err = router.route(context.Background(), group, model, map[int64]struct{}{11: {}})
	require.NoError(t, err)
	require.EqualValues(t, 10, route.account.ID)
	route.release()
}
