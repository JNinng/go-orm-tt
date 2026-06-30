// Package plugin 提供 GORM 插件示例，包括 Row-Level Security (RLS) 租户隔离。
//
// 本包中的 TenantPlugin 演示了如何通过 GORM 回调机制，
// 在每次数据库操作前自动注入租户上下文，
// 配合 PostgreSQL RLS (Row-Level Security) 策略实现多租户数据隔离。
package plugin

import (
	"context"

	"gorm.io/gorm"
)

// ============================================================
// 上下文键及辅助函数
// ============================================================

// tenantKey 是用于在 context.Context 中存储租户 ID 的私有键类型。
// 使用私有类型可避免与外部包的键冲突。
type tenantKey struct{}

// WithTenantID 将租户 ID 注入到 context 中，供 TenantPlugin 在回调中读取。
//
// 典型用法：
//
//	ctx := plugin.WithTenantID(context.Background(), "tenant-001")
//	db.WithContext(ctx).Find(&users)
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantKey{}, tenantID)
}

// TenantIDFromContext 从 context 中提取租户 ID。
// 返回空字符串表示未设置租户 ID。
func TenantIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(tenantKey{}).(string)
	return v
}

// ============================================================
// TenantPlugin — GORM RLS 插件
// ============================================================

// TenantPlugin 是一个 GORM 插件，在每次 CRUD 操作前通过
// PostgreSQL 的 set_config() 函数设置租户上下文变量 (app.tenant_id)，
// 配合数据库 RLS 策略实现行级别的租户数据隔离。
//
// 使用方式：
//
//	db, _ := gorm.Open(postgres.Open(dsn), &gorm.Config{})
//	db.Use(&plugin.TenantPlugin{})
//
//	// 在业务层注入租户 ID
//	ctx := plugin.WithTenantID(context.Background(), "t-123")
//	db.WithContext(ctx).Find(&users) // 回调自动注入 set_config
//
// RLS 策略示意（需在数据库中预先创建）：
//
//	ALTER TABLE users ENABLE ROW LEVEL SECURITY;
//	CREATE POLICY tenant_isolation ON users
//	    USING (tenant_id = current_setting('app.tenant_id'));
type TenantPlugin struct{}

// Name 返回插件名称，遵循 "rls:<维度>" 命名约定。
func (p *TenantPlugin) Name() string {
	return "tenant_rls"
}

// Initialize 注册 GORM 回调，在每次数据库操作前注入租户上下文。
//
// 回调注册在 Before("*") 阶段，覆盖 Create、Query、Update、Delete 四种操作。
// 每个回调会从 db.Statement.Context 中读取租户 ID，
// 并通过 PostgreSQL 的 set_config() 函数将其注入到会话本地变量中，
// 供数据库 RLS 策略使用。
//
// 为什么用 set_config() 而非 SET LOCAL：
// PostgreSQL 的 SET 命令不支持参数化查询（$1 占位符），
// 而 set_config(setting_name, new_value, is_local) 是普通函数，
// 可以安全地使用 GORM 的参数绑定，同时避免 SQL 注入风险。
// 第三个参数 true 表示 LOCAL 语义——变量仅在当前事务内有效。
func (p *TenantPlugin) Initialize(db *gorm.DB) error {
	// inject 从 context 读取 tenantID，通过 set_config 注入会话变量。
	// set_config 第三个参数 is_local=true 等价于 SET LOCAL，事务结束后自动清除。
	inject := func(db *gorm.DB) {
		tenantID, ok := db.Statement.Context.Value(tenantKey{}).(string)
		if !ok || tenantID == "" {
			return
		}
		db.Exec("SELECT set_config('app.tenant_id', ?, true)", tenantID)
	}

	// 对所有表（"*"）的四种操作注册回调。
	// 回调只在实际执行 SQL 前触发，性能开销极小（单次函数调用）。
	db.Callback().Create().Before("*").Register("rls:create", inject)
	db.Callback().Query().Before("*").Register("rls:query", inject)
	db.Callback().Update().Before("*").Register("rls:update", inject)
	db.Callback().Delete().Before("*").Register("rls:delete", inject)

	return nil
}
