package gormtest

import (
	"context"
	"errors"
	"testing"

	"go_orm_tt/internal/model"
	"go_orm_tt/internal/query"
	"go_orm_tt/test/dbtest"

	"gorm.io/gorm"
)

// cleanupTxTestData 清理事务测试中使用的用户数据（ID >= 40000）。
// 事务测试使用 MustDB（真实提交），需要确保每次运行时数据干净。
func cleanupTxTestData(t *testing.T, db *gorm.DB) {
	t.Helper()

	// 硬删除测试数据，绕过软删除
	if err := db.Unscoped().Where("id >= ? AND id < ?", 40000, 50000).Delete(&model.SysUser{}).Error; err != nil {
		t.Fatalf("清理测试数据失败: %v", err)
	}
}

// TestGORM_Transaction_Commit 测试事务提交。
func TestGORM_Transaction_Commit(t *testing.T) {
	db := dbtest.MustDB(t)
	cleanupTxTestData(t, db)
	t.Cleanup(func() { cleanupTxTestData(t, db) })
	ctx := context.Background()

	// 使用全局 DB（非事务级别的），手动管理事务
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		q := query.Use(tx)
		users := []*model.SysUser{
			{ID: 40100, Name: "tx_commit_1", Nickname: "事务提交测试1", Password: "p", TenantID: "default"},
			{ID: 40101, Name: "tx_commit_2", Nickname: "事务提交测试2", Password: "p", TenantID: "default"},
		}
		for _, u := range users {
			if err := q.SysUser.WithContext(ctx).Create(u); err != nil {
				return err
			}
		}
		return nil // 提交
	})
	if err != nil {
		t.Fatalf("事务提交失败: %v", err)
	}

	// 验证数据已提交
	q := query.Use(db)
	for _, id := range []int64{40100, 40101} {
		u, err := q.SysUser.WithContext(ctx).Where(q.SysUser.ID.Eq(id)).First()
		if err != nil {
			t.Errorf("事务提交后查询用户 %d 失败: %v", id, err)
		}
		if u == nil {
			t.Errorf("事务提交后用户 %d 不存在", id)
		}
	}

	// 清理
	for _, id := range []int64{40100, 40101} {
		_, _ = q.SysUser.WithContext(ctx).Where(q.SysUser.ID.Eq(id)).Delete()
	}
}

// TestGORM_Transaction_Rollback 测试事务回滚。
func TestGORM_Transaction_Rollback(t *testing.T) {
	db := dbtest.MustDB(t)
	cleanupTxTestData(t, db)
	t.Cleanup(func() { cleanupTxTestData(t, db) })
	ctx := context.Background()

	expectedErr := errors.New("rollback for test")
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		q := query.Use(tx)

		if err := q.SysUser.WithContext(ctx).Create(&model.SysUser{
			ID: 40200, Name: "tx_rollback", Nickname: "事务回滚测试", Password: "p", TenantID: "default",
		}); err != nil {
			return err
		}

		// 返回 error 应触发回滚
		return expectedErr
	})

	if !errors.Is(err, expectedErr) {
		t.Fatalf("期望错误 %v, 得到 %v", expectedErr, err)
	}

	// 验证数据已被回滚
	q := query.Use(db)
	u, err := q.SysUser.WithContext(ctx).Where(q.SysUser.ID.Eq(40200)).First()
	if err == nil {
		if u != nil {
			t.Error("事务回滚后仍然查询到用户，期望数据未提交")
		}
	}
}

// TestGORM_Transaction_Nested 测试嵌套事务（SavePoint）。
func TestGORM_Transaction_Nested(t *testing.T) {
	db := dbtest.MustDB(t)
	cleanupTxTestData(t, db)
	t.Cleanup(func() { cleanupTxTestData(t, db) })
	ctx := context.Background()

	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		q := query.Use(tx)

		// 外层创建用户
		if err := q.SysUser.WithContext(ctx).Create(&model.SysUser{
			ID: 40300, Name: "tx_outer", Nickname: "外层事务", Password: "p", TenantID: "default",
		}); err != nil {
			return err
		}

		// 嵌套事务：部分回滚
		tx.Transaction(func(tx2 *gorm.DB) error {
			q2 := query.Use(tx2)
			if err := q2.SysUser.WithContext(ctx).Create(&model.SysUser{
				ID: 40301, Name: "tx_inner", Nickname: "内层事务(回滚)", Password: "p", TenantID: "default",
			}); err != nil {
				return err
			}
			return errors.New("内层回滚")
		})

		// 继续在外层创建用户
		if err := q.SysUser.WithContext(ctx).Create(&model.SysUser{
			ID: 40302, Name: "tx_outer_2", Nickname: "外层事务2", Password: "p", TenantID: "default",
		}); err != nil {
			return err
		}

		return nil // 外层提交
	})
	if err != nil {
		t.Fatalf("嵌套事务失败: %v", err)
	}

	// 验证：外层数据应存在，内层已回滚
	q := query.Use(db)
	check := func(id int64, shouldExist bool) {
		u, err := q.SysUser.WithContext(ctx).Where(q.SysUser.ID.Eq(id)).First()
		if shouldExist && err != nil {
			t.Errorf("用户 %d 应存在但查询失败: %v", id, err)
		}
		if !shouldExist && err == nil && u != nil {
			t.Errorf("用户 %d 应回滚但仍然存在", id)
		}
	}
	check(40300, true)  // 外层 - 应存在
	check(40301, false) // 内层 - 已回滚
	check(40302, true)  // 外层 - 应存在

	// 清理
	for _, id := range []int64{40300, 40302} {
		_, _ = q.SysUser.WithContext(ctx).Where(q.SysUser.ID.Eq(id)).Delete()
	}
}

// TestGORM_Transaction_Savepoint 测试显式 Savepoint / RollbackTo。
func TestGORM_Transaction_Savepoint(t *testing.T) {
	db := dbtest.MustDB(t)
	cleanupTxTestData(t, db)
	t.Cleanup(func() { cleanupTxTestData(t, db) })
	ctx := context.Background()

	tx := db.WithContext(ctx).Begin()

	// 插入一条数据
	q := query.Use(tx)
	if err := q.SysUser.WithContext(ctx).Create(&model.SysUser{
		ID: 40400, Name: "tx_sp_1", Nickname: "Savepoint测试1", Password: "p", TenantID: "default",
	}); err != nil {
		tx.Rollback()
		t.Fatalf("创建用户失败: %v", err)
	}

	// 创建 Savepoint
	tx.SavePoint("sp1")

	// 在 Savepoint 之后插入第二条数据
	if err := q.SysUser.WithContext(ctx).Create(&model.SysUser{
		ID: 40401, Name: "tx_sp_2", Nickname: "Savepoint测试2", Password: "p", TenantID: "default",
	}); err != nil {
		tx.Rollback()
		t.Fatalf("创建用户失败: %v", err)
	}

	// 回滚到 Savepoint（应撤消第二条数据的插入）
	if err := tx.RollbackTo("sp1").Error; err != nil {
		tx.Rollback()
		t.Fatalf("RollbackTo 失败: %v", err)
	}

	// 提交
	if err := tx.Commit().Error; err != nil {
		t.Fatalf("Commit 失败: %v", err)
	}

	// 验证：第一条应存在，第二条应被回滚
	q2 := query.Use(db)
	u1, _ := q2.SysUser.WithContext(ctx).Where(q2.SysUser.ID.Eq(40400)).First()
	u2, _ := q2.SysUser.WithContext(ctx).Where(q2.SysUser.ID.Eq(40401)).First()

	if u1 == nil {
		t.Error("Savepoint 前插入的用户不存在")
	}
	if u2 != nil {
		t.Error("Savepoint 回滚后用户仍存在，期望已回滚")
	}

	// 清理
	if u1 != nil {
		_, _ = q2.SysUser.WithContext(ctx).Where(q2.SysUser.ID.Eq(40400)).Delete()
	}
}

// TestGORM_Transaction_Conflict 测试事务冲突。
func TestGORM_Transaction_Conflict(t *testing.T) {
	db := dbtest.MustDB(t)
	cleanupTxTestData(t, db)
	t.Cleanup(func() { cleanupTxTestData(t, db) })
	ctx := context.Background()

	// 准备数据
	q := query.Use(db)
	_ = q.SysUser.WithContext(ctx).Create(&model.SysUser{
		ID: 40500, Name: "tx_conflict", Nickname: "冲突测试", Password: "p", TenantID: "default",
	})

	// 在事务中尝试插入相同主键
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		qt := query.Use(tx)
		if err := qt.SysUser.WithContext(ctx).Create(&model.SysUser{
			ID: 40500, Name: "tx_conflict_dup", Nickname: "冲突测试(重复)", Password: "p", TenantID: "default",
		}); err != nil {
			return err
		}
		return nil
	})

	if err == nil {
		t.Error("期望主键冲突错误，但事务提交成功")
	} else {
		t.Logf("主键冲突正确捕获: %v", err)
	}

	// 验证原数据未被覆盖
	u, _ := q.SysUser.WithContext(ctx).Where(q.SysUser.ID.Eq(40500)).First()
	if u != nil && u.Name != "tx_conflict" {
		t.Errorf("原数据被覆盖: Name = %q, 期望 %q", u.Name, "tx_conflict")
	}

	// 清理
	_, _ = q.SysUser.WithContext(ctx).Where(q.SysUser.ID.Eq(40500)).Delete()
}

// TestGORM_Transaction_GenAPI 测试使用 gen Query.Transaction 方法。
func TestGORM_Transaction_GenAPI(t *testing.T) {
	db := dbtest.MustDB(t)
	cleanupTxTestData(t, db)
	t.Cleanup(func() { cleanupTxTestData(t, db) })
	ctx := context.Background()

	q := query.Use(db)

	// 使用 gen 自带的 Transaction 方法
	err := q.Transaction(func(tx *query.Query) error {
		if err := tx.SysUser.WithContext(ctx).Create(&model.SysUser{
			ID: 40600, Name: "gen_tx", Nickname: "Gen事务测试", Password: "p", TenantID: "default",
		}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("gen Transaction 失败: %v", err)
	}

	// 验证
	u, _ := q.SysUser.WithContext(ctx).Where(q.SysUser.ID.Eq(40600)).First()
	if u == nil {
		t.Error("gen Transaction 提交后查不到用户")
	}

	// 清理
	_, _ = q.SysUser.WithContext(ctx).Where(q.SysUser.ID.Eq(40600)).Delete()
}

// TestGORM_Transaction_Manual 测试手动 Begin / Commit / Rollback。
func TestGORM_Transaction_Manual(t *testing.T) {
	db := dbtest.MustDB(t)
	cleanupTxTestData(t, db)
	t.Cleanup(func() { cleanupTxTestData(t, db) })
	ctx := context.Background()

	// 手动管理事务：成功提交
	tx := db.WithContext(ctx).Begin()
	qt := query.Use(tx)

	if err := qt.SysUser.WithContext(ctx).Create(&model.SysUser{
		ID: 40700, Name: "manual_tx", Nickname: "手动事务", Password: "p", TenantID: "default",
	}); err != nil {
		tx.Rollback()
		t.Fatalf("创建用户失败: %v", err)
	}

	if err := tx.Commit().Error; err != nil {
		t.Fatalf("Commit 失败: %v", err)
	}

	// 验证提交成功
	q := query.Use(db)
	u, _ := q.SysUser.WithContext(ctx).Where(q.SysUser.ID.Eq(40700)).First()
	if u == nil {
		t.Error("手动 Commit 后查不到用户")
	}

	// 手动管理事务：回滚
	tx2 := db.WithContext(ctx).Begin()
	qt2 := query.Use(tx2)
	_ = qt2.SysUser.WithContext(ctx).Create(&model.SysUser{
		ID: 40701, Name: "manual_rollback", Nickname: "手动回滚", Password: "p", TenantID: "default",
	})
	tx2.Rollback()

	// 验证回滚
	u2, _ := q.SysUser.WithContext(ctx).Where(q.SysUser.ID.Eq(40701)).First()
	if u2 != nil {
		t.Error("Rollback 后仍存在用户，期望已回滚")
	}

	// 清理
	_, _ = q.SysUser.WithContext(ctx).Where(q.SysUser.ID.Eq(40700)).Delete()
}
