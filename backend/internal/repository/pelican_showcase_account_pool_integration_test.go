//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	dbmigrations "github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestPelicanAccountPoolRepository(t *testing.T) {
	ctx := context.Background()
	repo := NewPelicanGroupTestRepository(integrationDB)
	groups := createPelicanGroupTestGroups(t, "pool", "other")
	groupID := groups[0].ID
	client := testEntClient(t)
	a := mustCreateAccount(t, client, &service.Account{Name: "pool-selected"})
	b := mustCreateAccount(t, client, &service.Account{Name: "pool-outside"})
	t.Cleanup(func() { _, _ = integrationDB.ExecContext(ctx, `DELETE FROM accounts WHERE id IN ($1, $2)`, a.ID, b.ID) })
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO account_groups (account_id, group_id) VALUES ($1, $2), ($3, $4)`, a.ID, groupID, b.ID, groups[1].ID)
	require.NoError(t, err)
	pool, err := repo.GetPelicanAccountPool(ctx, groupID)
	require.NoError(t, err)
	require.Equal(t, service.PelicanAccountPoolAutomatic, pool.Mode)
	require.Empty(t, pool.AccountIDs)
	selected := &service.PelicanAccountPool{Mode: service.PelicanAccountPoolSelected, AccountIDs: []int64{a.ID}}
	require.NoError(t, repo.SavePelicanAccountPool(ctx, groupID, selected))
	pool, err = repo.GetPelicanAccountPool(ctx, groupID)
	require.NoError(t, err)
	require.Equal(t, selected, pool)
	require.ErrorIs(t, repo.SavePelicanAccountPool(ctx, groupID, &service.PelicanAccountPool{Mode: service.PelicanAccountPoolSelected, AccountIDs: []int64{b.ID}}), service.ErrPelicanAccountPoolInvalid)
	pool, err = repo.GetPelicanAccountPool(ctx, groupID)
	require.NoError(t, err)
	require.Equal(t, selected, pool, "错误保存不能覆盖已有配置")
	accounts, err := repo.ListPelicanPoolAccounts(ctx, groupID)
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	require.Equal(t, a.ID, accounts[0].ID)
	valid, err := repo.PelicanPoolAccountInGroup(ctx, groupID, a.ID)
	require.NoError(t, err)
	require.True(t, valid)
	_, err = integrationDB.ExecContext(ctx, `DELETE FROM account_groups WHERE account_id = $1`, a.ID)
	require.NoError(t, err)
	valid, err = repo.PelicanPoolAccountInGroup(ctx, groupID, a.ID)
	require.NoError(t, err)
	require.False(t, valid)
	pool, err = repo.GetPelicanAccountPool(ctx, groupID)
	require.NoError(t, err)
	require.Equal(t, selected, pool, "账号移组后保持指定模式，不回退全组")
	require.NoError(t, repo.SavePelicanAccountPool(ctx, groupID, &service.PelicanAccountPool{Mode: service.PelicanAccountPoolAutomatic}))
	pool, err = repo.GetPelicanAccountPool(ctx, groupID)
	require.NoError(t, err)
	require.Equal(t, service.PelicanAccountPoolAutomatic, pool.Mode)
	// 新迁移可以重复应用。
	sql, err := dbmigrations.FS.ReadFile("296_pelican_showcase_account_pools.sql")
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, string(sql))
	require.NoError(t, err)
}
