package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

func (r *pelicanGroupTestRepository) GetPelicanAccountPool(ctx context.Context, groupID int64) (*service.PelicanAccountPool, error) {
	ids := []int64{}
	err := r.db.QueryRowContext(ctx, `SELECT account_ids FROM pelican_showcase_account_pools WHERE group_id = $1`, groupID).Scan(pq.Array(&ids))
	if errors.Is(err, sql.ErrNoRows) {
		return &service.PelicanAccountPool{Mode: service.PelicanAccountPoolAutomatic, AccountIDs: ids}, nil
	}
	if err != nil {
		return nil, err
	}
	return &service.PelicanAccountPool{Mode: service.PelicanAccountPoolSelected, AccountIDs: ids}, nil
}

func (r *pelicanGroupTestRepository) SavePelicanAccountPool(ctx context.Context, groupID int64, pool *service.PelicanAccountPool) error {
	if pool.Mode == service.PelicanAccountPoolAutomatic {
		_, err := r.db.ExecContext(ctx, `DELETE FROM pelican_showcase_account_pools WHERE group_id = $1`, groupID)
		return err
	}
	result, err := r.db.ExecContext(ctx, `INSERT INTO pelican_showcase_account_pools (group_id, account_ids)
 SELECT g.id, $2::bigint[] FROM groups g WHERE g.id = $1 AND g.deleted_at IS NULL
 AND cardinality($2::bigint[]) = (
   SELECT COUNT(*) FROM account_groups ag JOIN accounts a ON a.id = ag.account_id
   WHERE ag.group_id = g.id AND a.deleted_at IS NULL AND a.id = ANY($2::bigint[])
 )
 ON CONFLICT (group_id) DO UPDATE SET account_ids = EXCLUDED.account_ids, updated_at = NOW()`, groupID, pq.Array(pool.AccountIDs))
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return service.ErrPelicanAccountPoolInvalid
	}
	return nil
}

func (r *pelicanGroupTestRepository) ListPelicanPoolAccounts(ctx context.Context, groupID int64) ([]service.PelicanPoolAccount, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT a.id, a.name, a.platform, a.status, a.schedulable
 FROM accounts a JOIN account_groups ag ON ag.account_id = a.id
 WHERE ag.group_id = $1 AND a.deleted_at IS NULL ORDER BY a.name, a.id`, groupID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	accounts := []service.PelicanPoolAccount{}
	for rows.Next() {
		var account service.PelicanPoolAccount
		if err := rows.Scan(&account.ID, &account.Name, &account.Platform, &account.Status, &account.Schedulable); err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	return accounts, rows.Err()
}

func (r *pelicanGroupTestRepository) PelicanPoolAccountInGroup(ctx context.Context, groupID, accountID int64) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS (
 SELECT 1 FROM account_groups ag JOIN accounts a ON a.id = ag.account_id
 WHERE ag.group_id = $1 AND a.id = $2 AND a.deleted_at IS NULL
 )`, groupID, accountID).Scan(&exists)
	return exists, err
}
