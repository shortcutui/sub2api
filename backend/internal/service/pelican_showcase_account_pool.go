package service

import (
	"context"
	"fmt"
	"slices"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	PelicanAccountPoolAutomatic   = "automatic"
	PelicanAccountPoolSelected    = "selected"
	PelicanAccountPoolMaxAccounts = 100
)

var ErrPelicanAccountPoolInvalid = infraerrors.BadRequest("INVALID_PELICAN_ACCOUNT_POOL", "请选择本分组内的 1–100 个账号，或切换为自动调度")

type PelicanAccountPool struct {
	Mode       string  `json:"mode"`
	AccountIDs []int64 `json:"account_ids"`
}

// 管理员选择账号所需的最小摘要，不返回凭据或其他账号配置。
type PelicanPoolAccount struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Platform    string `json:"platform"`
	Status      string `json:"status"`
	Schedulable bool   `json:"schedulable"`
}

type PelicanAccountPoolView struct {
	Pool     *PelicanAccountPool  `json:"pool"`
	Accounts []PelicanPoolAccount `json:"accounts"`
}

type PelicanAccountPoolRepository interface {
	GetPelicanAccountPool(context.Context, int64) (*PelicanAccountPool, error)
	SavePelicanAccountPool(context.Context, int64, *PelicanAccountPool) error
	ListPelicanPoolAccounts(context.Context, int64) ([]PelicanPoolAccount, error)
	PelicanPoolAccountInGroup(context.Context, int64, int64) (bool, error)
}

func (s *PelicanGroupTestService) GetAccountPool(ctx context.Context, groupID int64) (*PelicanAccountPoolView, error) {
	if group, err := s.groups.GetByID(ctx, groupID); err != nil {
		return nil, err
	} else if group == nil {
		return nil, ErrGroupNotFound
	}
	pool, err := s.repo.GetPelicanAccountPool(ctx, groupID)
	if err != nil {
		return nil, err
	}
	accounts, err := s.repo.ListPelicanPoolAccounts(ctx, groupID)
	if err != nil {
		return nil, err
	}
	return &PelicanAccountPoolView{Pool: pool, Accounts: accounts}, nil
}

func (s *PelicanGroupTestService) UpdateAccountPool(ctx context.Context, groupID int64, pool PelicanAccountPool) (*PelicanAccountPoolView, error) {
	if group, err := s.groups.GetByID(ctx, groupID); err != nil {
		return nil, err
	} else if group == nil {
		return nil, ErrGroupNotFound
	}
	switch pool.Mode {
	case PelicanAccountPoolAutomatic:
		pool.AccountIDs = []int64{}
	case PelicanAccountPoolSelected:
		if len(pool.AccountIDs) == 0 || len(pool.AccountIDs) > PelicanAccountPoolMaxAccounts {
			return nil, ErrPelicanAccountPoolInvalid
		}
		pool.AccountIDs = slices.Clone(pool.AccountIDs)
		slices.Sort(pool.AccountIDs)
		pool.AccountIDs = slices.Compact(pool.AccountIDs)
		if pool.AccountIDs[0] <= 0 {
			return nil, ErrPelicanAccountPoolInvalid
		}
	default:
		return nil, ErrPelicanAccountPoolInvalid
	}
	// 成员校验和持久化在同一条 SQL 中完成，避免错误名单覆盖原配置。
	if err := s.repo.SavePelicanAccountPool(ctx, groupID, &pool); err != nil {
		return nil, err
	}
	return s.GetAccountPool(ctx, groupID)
}

type pelicanAccountPoolKey struct{}
type pelicanAccountPoolScope struct {
	groupID int64
	ids     map[int64]struct{}
}

// 每轮读取一次配置，所有并行输出和重试共用该快照。读失败时停止本轮。
func (s *PelicanGroupTestService) withAccountPool(ctx context.Context, groupID int64) (context.Context, error) {
	pool, err := s.repo.GetPelicanAccountPool(ctx, groupID)
	if err != nil {
		return ctx, err
	}
	if pool == nil {
		return ctx, ErrPelicanAccountPoolInvalid
	}
	if pool.Mode == PelicanAccountPoolAutomatic {
		return ctx, nil
	}
	if pool.Mode != PelicanAccountPoolSelected || len(pool.AccountIDs) == 0 {
		return ctx, ErrPelicanAccountPoolInvalid
	}
	accounts, err := s.repo.ListPelicanPoolAccounts(ctx, groupID)
	if err != nil {
		return ctx, err
	}
	scope := &pelicanAccountPoolScope{groupID: groupID, ids: make(map[int64]struct{})}
	for _, account := range accounts {
		if slices.Contains(pool.AccountIDs, account.ID) {
			scope.ids[account.ID] = struct{}{}
		}
	}
	if len(scope.ids) == 0 {
		return ctx, fmt.Errorf("展示账号池没有仍属于本分组的账号")
	}
	return context.WithValue(ctx, pelicanAccountPoolKey{}, scope), nil
}

// 在调度器评分和抢并发槽之前过滤，普通请求没有此上下文，不受影响。
func pelicanShowcaseAccountAllowed(ctx context.Context, account *Account) bool {
	scope, _ := ctx.Value(pelicanAccountPoolKey{}).(*pelicanAccountPoolScope)
	if scope == nil {
		return true
	}
	if account == nil {
		return false
	}
	_, allowed := scope.ids[account.ID]
	return allowed
}

// 发起模型调用前复核成员关系，避免调度缓存仍包含已移组或删除的账号。
func (s *PelicanGroupTestService) routePelicanAccount(ctx context.Context, group *Group, model string, excluded map[int64]struct{}) (*pelicanGroupRoute, error) {
	scope, _ := ctx.Value(pelicanAccountPoolKey{}).(*pelicanAccountPoolScope)
	if scope == nil {
		return s.router.route(ctx, group, model, excluded)
	}
	for range len(scope.ids) + 1 {
		route, err := s.router.route(ctx, group, model, excluded)
		if err != nil {
			return nil, err
		}
		if !pelicanShowcaseAccountAllowed(ctx, route.account) {
			route.release()
			return nil, fmt.Errorf("调度器返回了展示账号池以外的账号")
		}
		valid, err := s.repo.PelicanPoolAccountInGroup(ctx, scope.groupID, route.account.ID)
		if err == nil && valid {
			return route, nil
		}
		route.release()
		if err != nil {
			return nil, err
		}
		excluded[route.account.ID] = struct{}{}
	}
	return nil, ErrNoAvailableAccounts
}
