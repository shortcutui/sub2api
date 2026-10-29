-- 展示账号池独立于业务分组和测试计划；没有配置行表示沿用自动调度。
CREATE TABLE IF NOT EXISTS pelican_showcase_account_pools (
    group_id BIGINT PRIMARY KEY REFERENCES groups(id) ON DELETE CASCADE,
    account_ids BIGINT[] NOT NULL CHECK (cardinality(account_ids) BETWEEN 1 AND 100),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE pelican_showcase_account_pools IS
    '鹈鹕展示分组的指定账号池，同组全部绘图计划共用；账号失效时不得回退到全组';
