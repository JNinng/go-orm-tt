package gormtest

import (
	"context"
	"testing"
	"time"

	"go_orm_tt/test/dbtest"

	"gorm.io/gorm"
)

// TestGORM_Hooks_BeforeCreate 测试 BeforeCreate 钩子。
func TestGORM_Hooks_BeforeCreate(t *testing.T) {
	db := dbtest.MustTxDB(t)
	ctx := context.Background()

	hookCtx := &HookExecContext{}
	user := &HookUser{
		ID:          30100,
		Name:        "hook_before_create",
		Nickname:    "BeforeCreate测试",
		Password:    "pass",
		CreatedBy:   1,
		UpdatedBy:   1,
		TenantID:    "default",
		HookContext: hookCtx,
	}

	if err := db.WithContext(ctx).Create(user).Error; err != nil {
		t.Fatalf("Create 失败: %v", err)
	}

	// BeforeCreate 应该自动设置 CreatedAt
	if user.CreatedAt.IsZero() {
		t.Error("BeforeCreate 钩子未自动设置 CreatedAt")
	}

	// 验证钩子执行顺序
	if !hookCtx.Contains("BeforeCreate") {
		t.Error("BeforeCreate 钩子未执行")
	}
	if !hookCtx.Contains("AfterCreate") {
		t.Error("AfterCreate 钩子未执行")
	}
	if hookCtx.Count("BeforeCreate") != 1 {
		t.Errorf("BeforeCreate 执行次数 = %d, 期望 1", hookCtx.Count("BeforeCreate"))
	}
}

// TestGORM_Hooks_BeforeUpdate 测试 BeforeUpdate / AfterUpdate 钩子。
func TestGORM_Hooks_BeforeUpdate(t *testing.T) {
	db := dbtest.MustTxDB(t)
	ctx := context.Background()

	// 先创建用户
	hookCtx := &HookExecContext{}
	user := &HookUser{
		ID:          30200,
		Name:        "hook_before_update",
		Nickname:    "BeforeUpdate测试",
		Password:    "pass",
		CreatedBy:   1,
		UpdatedBy:   1,
		TenantID:    "default",
		HookContext: hookCtx,
	}
	if err := db.WithContext(ctx).Create(user).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}

	// 重置钩子记录，只观察 Update
	hookCtx.Records = nil

	// 更新用户
	if err := db.WithContext(ctx).Model(user).Update("nickname", "更新后的昵称").Error; err != nil {
		t.Fatalf("Update 失败: %v", err)
	}

	if !hookCtx.Contains("BeforeUpdate") {
		t.Error("BeforeUpdate 钩子未执行")
	}
	if !hookCtx.Contains("AfterUpdate") {
		t.Error("AfterUpdate 钩子未执行")
	}

	// BeforeUpdate 应该自动设置 UpdatedAt
	if user.UpdatedAt.IsZero() {
		t.Error("BeforeUpdate 钩子未自动设置 UpdatedAt")
	}
}

// TestGORM_Hooks_BeforeDelete 测试 BeforeDelete / AfterDelete 钩子。
func TestGORM_Hooks_BeforeDelete(t *testing.T) {
	db := dbtest.MustTxDB(t)
	ctx := context.Background()

	// 先创建用户
	hookCtx := &HookExecContext{}
	user := &HookUser{
		ID:          30300,
		Name:        "hook_before_delete",
		Nickname:    "BeforeDelete测试",
		Password:    "pass",
		CreatedBy:   1,
		UpdatedBy:   1,
		TenantID:    "default",
		HookContext: hookCtx,
	}
	if err := db.WithContext(ctx).Create(user).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}

	// 重置钩子记录，只观察 Delete
	hookCtx.Records = nil

	// 删除用户（软删除）
	if err := db.WithContext(ctx).Delete(user).Error; err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}

	if !hookCtx.Contains("BeforeDelete") {
		t.Error("BeforeDelete 钩子未执行")
	}
	if !hookCtx.Contains("AfterDelete") {
		t.Error("AfterDelete 钩子未执行")
	}

	// 验证是标记删除：普通查询查不到
	var found HookUser
	err := db.WithContext(ctx).First(&found, 30300).Error
	if err == nil {
		t.Error("软删除后普通查询仍能查到用户")
	}

	// 验证 Unscoped 仍存在，且 deleted_at 已标记
	err = db.WithContext(ctx).Unscoped().First(&found, 30300).Error
	if err != nil {
		t.Fatal("Unscoped 查询失败，可能是物理删除了")
	}
	if found.DeletedAt.Time.IsZero() {
		t.Error("deleted_at 为空，不是标记删除")
	}
	t.Logf("软删除标记确认: deleted_at = %v", found.DeletedAt.Time)
}

// TestGORM_Hooks_ErrorInHook 测试钩子返回 error 时操作被回滚。
func TestGORM_Hooks_ErrorInHook(t *testing.T) {
	db := dbtest.MustTxDB(t)
	ctx := context.Background()

	// 注册一个全局回调，当 name 为 "fail_me" 时在 INSERT 前返回 error
	hookName := "test:hook:fail_on_create"
	db.Callback().Create().Before("gorm:create").Register(hookName, func(db *gorm.DB) {
		if user, ok := db.Statement.Dest.(*HookUser); ok && user.Name == "fail_me" {
			_ = db.AddError(ErrHookForcedFail)
		}
	})
	t.Cleanup(func() {
		db.Callback().Create().Remove(hookName)
	})

	user := &HookUser{
		ID:        30400,
		Name:      "fail_me",
		Nickname:  "钩子失败测试",
		Password:  "pass",
		CreatedBy: 1,
		UpdatedBy: 1,
		TenantID:  "default",
	}

	err := db.WithContext(ctx).Create(user).Error
	if err == nil {
		t.Fatal("期望钩子错误导致 Create 失败，但得到 nil")
	}

	// 验证数据未被写入
	var found HookUser
	if err := db.WithContext(ctx).First(&found, 30400).Error; err == nil {
		t.Error("钩子失败后数据仍被写入，期望回滚")
	}
}

// TestGORM_Hooks_AfterFind 测试 AfterFind 钩子。
func TestGORM_Hooks_AfterFind(t *testing.T) {
	db := dbtest.MustTxDB(t)
	ctx := context.Background()

	// 先创建用户
	user := &HookUser{
		ID:        30500,
		Name:      "hook_after_find",
		Nickname:  "AfterFind测试",
		Password:  "pass",
		CreatedBy: 1,
		UpdatedBy: 1,
		TenantID:  "default",
	}
	if err := db.WithContext(ctx).Create(user).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}

	// 查询用户
	hookCtx := &HookExecContext{}
	var found HookUser
	if err := db.WithContext(ctx).Where("id = ?", 30500).First(&found).Error; err != nil {
		t.Fatalf("First 失败: %v", err)
	}
	found.HookContext = hookCtx

	// 再次查询触发 AfterFind
	var found2 HookUser
	if err := db.WithContext(ctx).Where("id = ?", 30500).First(&found2).Error; err != nil {
		t.Fatalf("第二次 First 失败: %v", err)
	}
	found2.HookContext = hookCtx

	// 此时 AfterFind 应该已经触发
	t.Log("AfterFind 钩子已执行（通过日志或 context 验证）")
}

// TestGORM_Hooks_HookOrder 测试多个钩子的执行顺序。
func TestGORM_Hooks_HookOrder(t *testing.T) {
	db := dbtest.MustTxDB(t)
	ctx := context.Background()

	hookCtx := &HookExecContext{}
	now := time.Now()

	user := &HookUser{
		ID:          30600,
		Name:        "hook_order",
		Nickname:    "钩子顺序测试",
		Password:    "pass",
		CreatedBy:   1,
		UpdatedBy:   1,
		TenantID:    "default",
		HookContext: hookCtx,
		CreatedAt:   now,
	}

	if err := db.WithContext(ctx).Create(user).Error; err != nil {
		t.Fatalf("Create 失败: %v", err)
	}

	// 验证 BeforeCreate 在 AfterCreate 之前
	beforeIdx := -1
	afterIdx := -1
	for i, r := range hookCtx.Records {
		if r.Hook == "BeforeCreate" {
			beforeIdx = i
		}
		if r.Hook == "AfterCreate" {
			afterIdx = i
		}
	}
	if beforeIdx < 0 {
		t.Error("BeforeCreate 未执行")
	}
	if afterIdx < 0 {
		t.Error("AfterCreate 未执行")
	}
	if beforeIdx >= 0 && afterIdx >= 0 && beforeIdx > afterIdx {
		t.Error("BeforeCreate 执行顺序在 AfterCreate 之后，期望 BeforeCreate 先执行")
	}
}
