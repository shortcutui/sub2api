package admin

import (
	"context"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type accountPoolHandlerRepo struct {
	service.PelicanGroupTestRepository
	pool *service.PelicanAccountPool
}

func (r *accountPoolHandlerRepo) GetPelicanAccountPool(context.Context, int64) (*service.PelicanAccountPool, error) {
	return r.pool, nil
}
func (r *accountPoolHandlerRepo) SavePelicanAccountPool(_ context.Context, _ int64, pool *service.PelicanAccountPool) error {
	r.pool = pool
	return nil
}
func (*accountPoolHandlerRepo) ListPelicanPoolAccounts(context.Context, int64) ([]service.PelicanPoolAccount, error) {
	return []service.PelicanPoolAccount{{ID: 11, Name: "selected", Platform: "openai", Status: "active", Schedulable: true}}, nil
}

func TestPelicanAccountPoolHandler(t *testing.T) {
	repo := &accountPoolHandlerRepo{pool: &service.PelicanAccountPool{Mode: service.PelicanAccountPoolAutomatic, AccountIDs: []int64{}}}
	h := NewPelicanGroupTestHandler(service.NewPelicanGroupTestService(repo, groupTestHandlerGroups{}, nil, nil, nil, nil, nil, nil))
	route := "/groups/:id/account-pool"
	path := "/groups/4/account-pool"
	w := serveGroupTest(h.GetAccountPool, http.MethodGet, route, path, "")
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"mode":"automatic"`)
	require.Contains(t, w.Body.String(), `"name":"selected"`)
	w = serveGroupTest(h.UpdateAccountPool, http.MethodPut, route, path, `{"mode":"selected","account_ids":[11]}`)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"account_ids":[11]`)
	for _, body := range []string{`{`, `{"mode":"selected","account_ids":[]}`, `{"mode":"selected","account_ids":[1.5]}`, `{"mode":"selected","account_ids":["11"]}`} {
		require.Equal(t, http.StatusBadRequest, serveGroupTest(h.UpdateAccountPool, http.MethodPut, route, path, body).Code)
	}
	require.Equal(t, http.StatusBadRequest, serveGroupTest(h.GetAccountPool, http.MethodGet, route, "/groups/0/account-pool", "").Code)
	require.Equal(t, http.StatusNotFound, serveGroupTest(h.GetAccountPool, http.MethodGet, route, "/groups/99/account-pool", "").Code)
	w = serveGroupTest(h.UpdateAccountPool, http.MethodPut, route, path, `{"mode":"automatic","account_ids":[]}`)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"account_ids":[]`)
}
