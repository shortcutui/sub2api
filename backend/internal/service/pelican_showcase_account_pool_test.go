package service

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// 原有测试计划仓库默认没有展示账号池，保持原测试语义。
func (*groupTestRepoFake) GetPelicanAccountPool(context.Context, int64) (*PelicanAccountPool, error) {
	return &PelicanAccountPool{Mode: PelicanAccountPoolAutomatic, AccountIDs: []int64{}}, nil
}
func (*groupTestRepoFake) SavePelicanAccountPool(context.Context, int64, *PelicanAccountPool) error {
	return nil
}
func (*groupTestRepoFake) ListPelicanPoolAccounts(context.Context, int64) ([]PelicanPoolAccount, error) {
	return nil, nil
}
func (*groupTestRepoFake) PelicanPoolAccountInGroup(context.Context, int64, int64) (bool, error) {
	return true, nil
}

type poolTestRepo struct {
	*groupTestRepoFake
	pool     *PelicanAccountPool
	accounts []PelicanPoolAccount
	err      error
	removed  int64
}

func (r *poolTestRepo) GetPelicanAccountPool(context.Context, int64) (*PelicanAccountPool, error) {
	return r.pool, r.err
}
func (r *poolTestRepo) SavePelicanAccountPool(_ context.Context, _ int64, pool *PelicanAccountPool) error {
	r.pool = pool
	return nil
}
func (r *poolTestRepo) ListPelicanPoolAccounts(context.Context, int64) ([]PelicanPoolAccount, error) {
	return r.accounts, r.err
}
func (r *poolTestRepo) PelicanPoolAccountInGroup(_ context.Context, _, id int64) (bool, error) {
	return id != r.removed, r.err
}

func newPoolTestService(router pelicanGroupRouter, run func(context.Context, int64, string, *PelicanTestConfig) (*ScheduledTestResult, error)) (*PelicanGroupTestService, *poolTestRepo, *showcaseRepoStub) {
	base := newGroupTestRepoFake()
	svc, showcase := newGroupTestService(base, router, run)
	repo := &poolTestRepo{groupTestRepoFake: base, pool: &PelicanAccountPool{Mode: PelicanAccountPoolSelected, AccountIDs: []int64{11, 12}}, accounts: []PelicanPoolAccount{{ID: 11}, {ID: 12}}}
	svc.repo = repo
	return svc, repo, showcase
}

func TestPelicanAccountPoolValidation(t *testing.T) {
	svc, repo, _ := newPoolTestService(nil, nil)
	for _, pool := range []PelicanAccountPool{
		{}, {Mode: "unknown"}, {Mode: PelicanAccountPoolSelected},
		{Mode: PelicanAccountPoolSelected, AccountIDs: []int64{-1}},
		{Mode: PelicanAccountPoolSelected, AccountIDs: make([]int64, 101)},
	} {
		_, err := svc.UpdateAccountPool(context.Background(), 4, pool)
		require.ErrorIs(t, err, ErrPelicanAccountPoolInvalid)
	}
	_, err := svc.UpdateAccountPool(context.Background(), 4, PelicanAccountPool{Mode: PelicanAccountPoolSelected, AccountIDs: []int64{12, 11, 12}})
	require.NoError(t, err)
	require.Equal(t, []int64{11, 12}, repo.pool.AccountIDs)
	_, err = svc.UpdateAccountPool(context.Background(), 4, PelicanAccountPool{Mode: PelicanAccountPoolAutomatic, AccountIDs: []int64{11}})
	require.NoError(t, err)
	require.Empty(t, repo.pool.AccountIDs)
}

func TestPelicanAccountPoolRetriesAndPublishes(t *testing.T) {
	router := &groupTestRouterFake{steps: []routeStep{{account: account(11, "a")}, {account: account(12, "b")}}}
	svc, repo, showcase := newPoolTestService(router, answers(map[int64]ScheduledTestResult{
		11: {Status: "failed", ErrorMessage: "API returned 429"},
		12: {Status: "success", ResponseText: "<html>pelican</html>"},
	}))
	svc.execute(groupTestPlan(1), time.Now().Add(time.Minute))
	require.Len(t, repo.results, 1)
	require.Equal(t, "success", repo.results[0].Status)
	require.EqualValues(t, 12, repo.results[0].AccountID)
	require.EqualValues(t, 11, repo.results[0].Attempts[0].AccountID)
	require.Len(t, showcase.published, 1)
	require.Equal(t, "<html>pelican</html>", showcase.published[0].snapshot.ResponseText)
	require.ElementsMatch(t, []int64{11, 12}, router.released)
}

func TestPelicanAccountPoolExhaustionKeepsExistingArtwork(t *testing.T) {
	router := &groupTestRouterFake{steps: []routeStep{{account: account(11, "a")}, {account: account(12, "b")}}}
	svc, repo, showcase := newPoolTestService(router, answers(map[int64]ScheduledTestResult{
		11: {Status: "failed", ErrorMessage: "API returned 429"},
		12: {Status: "failed", ErrorMessage: "API returned 503"},
	}))
	showcase.published = []publishCall{{snapshot: PelicanShowcaseSnapshot{ResponseText: "<html>previous</html>"}}}
	svc.execute(groupTestPlan(1), time.Now().Add(time.Minute))
	require.Len(t, repo.results, 1)
	require.Equal(t, "failed", repo.results[0].Status)
	require.Len(t, showcase.published, 1)
	require.Equal(t, "<html>previous</html>", showcase.published[0].snapshot.ResponseText)
	require.Empty(t, showcase.pruned)
}

func TestPelicanAccountPoolIsSharedByGroupPlans(t *testing.T) {
	router := &groupTestRouterFake{steps: []routeStep{{account: account(11, "a")}, {account: account(12, "b")}}}
	svc, repo, _ := newPoolTestService(router, answers(map[int64]ScheduledTestResult{
		11: {Status: "success", ResponseText: "<html>a</html>"},
		12: {Status: "success", ResponseText: "<html>b</html>"},
	}))
	first, second := groupTestPlan(1), groupTestPlan(1)
	second.ID = 8
	svc.execute(first, time.Now().Add(time.Minute))
	svc.execute(second, time.Now().Add(time.Minute))
	require.Len(t, repo.results, 2)
	require.Equal(t, first.ID, repo.results[0].PlanID)
	require.Equal(t, second.ID, repo.results[1].PlanID)
	require.EqualValues(t, 11, repo.results[0].AccountID)
	require.EqualValues(t, 12, repo.results[1].AccountID)
}

func TestPelicanAccountPoolStopsOnReadFailureOrMissingMembers(t *testing.T) {
	for _, failRead := range []bool{false, true} {
		svc, repo, _ := newPoolTestService(nil, func(context.Context, int64, string, *PelicanTestConfig) (*ScheduledTestResult, error) {
			t.Fatal("不得调用模型")
			return nil, nil
		})
		repo.accounts = nil
		if failRead {
			repo.err = errors.New("database unavailable")
		}
		results := svc.runSamples(context.Background(), groupTestPlan(2))
		require.Len(t, results, 1)
		require.Equal(t, "failed", results[0].Status)
		require.Contains(t, results[0].ErrorMessage, "account_pool_unavailable")
	}
}

func TestPelicanAccountPoolRechecksRemovedAccount(t *testing.T) {
	router := &groupTestRouterFake{steps: []routeStep{{account: account(11, "removed")}, {account: account(12, "valid")}}}
	called := []int64{}
	svc, repo, _ := newPoolTestService(router, func(_ context.Context, id int64, _ string, _ *PelicanTestConfig) (*ScheduledTestResult, error) {
		called = append(called, id)
		return &ScheduledTestResult{Status: "success", ResponseText: "<html>pelican</html>"}, nil
	})
	repo.removed = 11
	results := svc.runSamples(context.Background(), groupTestPlan(1))
	require.Equal(t, "success", results[0].Status)
	require.Equal(t, []int64{12}, called)
	require.ElementsMatch(t, []int64{11, 12}, router.released)
}

func TestPelicanAccountPoolDefendsAgainstOutOfPoolSelection(t *testing.T) {
	router := &groupTestRouterFake{steps: []routeStep{{account: account(99, "outside")}}}
	svc, _, _ := newPoolTestService(router, func(context.Context, int64, string, *PelicanTestConfig) (*ScheduledTestResult, error) {
		t.Fatal("不得调用池外账号")
		return nil, nil
	})
	results := svc.runSamples(context.Background(), groupTestPlan(1))
	require.Equal(t, "failed", results[0].Status)
	require.Equal(t, []int64{99}, router.released)
}

func TestPelicanAccountPoolOpenAISchedulers(t *testing.T) {
	for _, advanced := range []bool{false, true} {
		t.Run(map[bool]string{false: "legacy", true: "advanced"}[advanced], func(t *testing.T) {
			resetOpenAIAdvancedSchedulerSettingCacheForTest()
			t.Cleanup(resetOpenAIAdvancedSchedulerSettingCacheForTest)
			group := &Group{ID: 4, Platform: PlatformOpenAI, Status: StatusActive}
			accounts := []Account{
				{ID: 10, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 0, GroupIDs: []int64{4}},
				{ID: 11, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 5, GroupIDs: []int64{4}},
				{ID: 12, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusDisabled, Schedulable: false, Concurrency: 1, GroupIDs: []int64{4}},
			}
			acquired, released := []int64{}, []int64{}
			cfg := &config.Config{}
			cfg.Gateway.Scheduling.LoadBatchEnabled = true
			concurrency := NewConcurrencyService(schedulerTestConcurrencyCache{acquiredIDs: &acquired, releasedIDs: &released})
			openai := &OpenAIGatewayService{accountRepo: schedulerTestOpenAIAccountRepo{accounts: accounts}, cache: &schedulerTestGatewayCache{}, cfg: cfg, rateLimitService: newOpenAIAdvancedSchedulerRateLimitService(strconv.FormatBool(advanced)), concurrencyService: concurrency}
			router := &gatewayPelicanGroupRouter{openai: openai, concurrency: concurrency, slotWait: time.Millisecond}
			svc, _, _ := newPoolTestService(router, nil)
			ctx, err := svc.withAccountPool(context.Background(), group.ID)
			require.NoError(t, err)
			route, err := router.route(ctx, group, "gpt-6-astra", nil)
			require.NoError(t, err)
			require.EqualValues(t, 11, route.account.ID)
			require.Equal(t, []int64{11}, acquired)
			route.release()
			_, err = router.route(ctx, group, "gpt-6-astra", map[int64]struct{}{11: {}})
			require.ErrorIs(t, err, ErrNoAvailableAccounts, "不可回退到池外账号或停用账号")
			selection, _, err := openai.SelectAccountWithSchedulerForCapability(context.Background(), &group.ID, "", "", "gpt-6-astra", map[int64]struct{}{11: {}}, OpenAIUpstreamTransportAny, OpenAIEndpointCapabilityChatCompletions, false, false, true)
			require.NoError(t, err)
			require.EqualValues(t, 10, selection.Account.ID, "普通请求仍能使用池外账号")
			selection.ReleaseFunc()
		})
	}
}
