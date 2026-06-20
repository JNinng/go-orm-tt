// Package suite 提供 ORM 无关的抽象测试接口和测试运行器。
//
// 每个 ORM 层（gorm、ent 等）只需实现 UserCRUD + UserFinder 接口，
// 即可通过 RunUserCRUDTests / RunUserFinderTests 运行标准测试用例。
//
// 设计目标：加一个新的 ORM 测试只需约 50 行适配代码。
package suite

import (
	"context"
	"testing"

	"go_orm_tt/internal/model"
)

// ============================================================
// 各 ORM 需实现的接口（使用现有 model.SysUser）
// ============================================================

// UserCRUD 用户基础 CRUD 操作接口。
type UserCRUD interface {
	// GetUser 按 ID 查询用户（含软删除过滤），不存在时返回 (nil, nil)。
	GetUser(ctx context.Context, id int64) (*model.SysUser, error)

	// GetUserUnscoped 按 ID 查询用户（忽略软删除），用于验证软删除标记。
	GetUserUnscoped(ctx context.Context, id int64) (*model.SysUser, error)

	// CreateUser 创建用户并返回完整结果（含数据库默认值）。
	CreateUser(ctx context.Context, user *model.SysUser) (*model.SysUser, error)

	// UpdateUser 更新用户的指定字段。
	UpdateUser(ctx context.Context, user *model.SysUser) error

	// DeleteUser 删除（软删除）用户。
	DeleteUser(ctx context.Context, id int64) error

	// ListUser 分页查询用户。
	ListUser(ctx context.Context, limit, offset int) ([]*model.SysUser, error)

	// BatchCreateUser 批量创建用户，返回影响行数。
	BatchCreateUser(ctx context.Context, users []*model.SysUser) (int64, error)
}

// UserFinder 批量查询接口。
type UserFinder interface {
	// FindUser 无条件查询所有用户。
	FindUser(ctx context.Context) ([]*model.SysUser, error)

	// FindUserInBatches 分批查询，每批调用一次 fn(batch, batchNo)。
	FindUserInBatches(ctx context.Context, batchSize int, fn func([]model.SysUser, int) error) error
}

// ============================================================
// 测试运行器
// ============================================================

// RunUserCRUDTests 运行基础 CRUD 测试。
func RunUserCRUDTests(t *testing.T, s UserCRUD, ctx context.Context) {
	t.Run("创建用户", func(t *testing.T) {
		user := &model.SysUser{
			ID:       10100,
			Status:   1,
			Name:     "crud_create",
			Nickname: "CRUD创建测试",
			Password: "pass",
			TenantID: "default",
			Remark:   "create test",
		}
		created, err := s.CreateUser(ctx, user)
		if err != nil {
			t.Fatalf("CreateUser 失败: %v", err)
		}
		if created == nil {
			t.Fatal("CreateUser 返回 nil")
		}
		if created.Name != user.Name {
			t.Errorf("Name = %q, 期望 %q", created.Name, user.Name)
		}
	})

	t.Run("查询已存在用户", func(t *testing.T) {
		got, err := s.GetUser(ctx, 10100)
		if err != nil {
			t.Fatalf("GetUser 失败: %v", err)
		}
		if got == nil {
			t.Fatal("GetUser 返回 nil")
		}
		if got.ID != 10100 {
			t.Errorf("ID = %d, 期望 10100", got.ID)
		}
	})

	t.Run("查询不存在的用户", func(t *testing.T) {
		got, err := s.GetUser(ctx, -1)
		if err != nil {
			t.Fatalf("GetUser 失败: %v", err)
		}
		if got != nil {
			t.Fatalf("期望 nil, 得到 %+v", got)
		}
	})

	t.Run("更新用户", func(t *testing.T) {
		err := s.UpdateUser(ctx, &model.SysUser{
			ID:       10100,
			Nickname: "更新的昵称",
			Remark:   "updated",
		})
		if err != nil {
			t.Fatalf("UpdateUser 失败: %v", err)
		}
		got, _ := s.GetUser(ctx, 10100)
		if got == nil {
			t.Fatal("更新后查不到用户")
		}
		if got.Nickname != "更新的昵称" {
			t.Errorf("Nickname = %q, 期望 %q", got.Nickname, "更新的昵称")
		}
	})

	t.Run("软删除用户", func(t *testing.T) {
		err := s.DeleteUser(ctx, 10100)
		if err != nil {
			t.Fatalf("DeleteUser 失败: %v", err)
		}

		// 验证软删除已生效：普通查询查不到
		got, _ := s.GetUser(ctx, 10100)
		if got != nil {
			t.Error("软删除后普通查询仍能查到用户")
		}

		// 验证是标记删除而非物理删除：Unscoped 能查到且 deleted_at 非空
		got, err = s.GetUserUnscoped(ctx, 10100)
		if err != nil {
			t.Fatalf("GetUserUnscoped 失败: %v", err)
		}
		if got == nil {
			t.Fatal("Unscoped 查询返回 nil，可能是物理删除了")
		}
		if got.DeletedAt.Time.IsZero() {
			t.Error("deleted_at 为空，不是标记删除，可能是物理删除")
		}
		t.Logf("软删除标记确认: deleted_at = %v", got.DeletedAt.Time)
	})

	t.Run("批量创建用户", func(t *testing.T) {
		users := []*model.SysUser{
			{ID: 10201, Name: "batch_1", Nickname: "批量1", Password: "p", TenantID: "default"},
			{ID: 10202, Name: "batch_2", Nickname: "批量2", Password: "p", TenantID: "default"},
			{ID: 10203, Name: "batch_3", Nickname: "批量3", Password: "p", TenantID: "default"},
		}
		affected, err := s.BatchCreateUser(ctx, users)
		if err != nil {
			t.Fatalf("BatchCreateUser 失败: %v", err)
		}
		if affected != 3 {
			t.Errorf("影响行数 = %d, 期望 3", affected)
		}
	})

	t.Run("分页查询用户", func(t *testing.T) {
		all, err := s.ListUser(ctx, 100, 0)
		if err != nil {
			t.Fatalf("ListUser 失败: %v", err)
		}
		if len(all) < 4 {
			t.Errorf("返回 %d 条，期望至少 4 条", len(all))
		}
	})
}

// RunUserFinderTests 运行批量查询测试。
func RunUserFinderTests(t *testing.T, s UserFinder, ctx context.Context) {
	t.Run("FindUser", func(t *testing.T) {
		users, err := s.FindUser(ctx)
		if err != nil {
			t.Fatalf("FindUser 失败: %v", err)
		}
		t.Logf("FindUser 返回 %d 条记录", len(users))
	})

	t.Run("FindUserInBatches", func(t *testing.T) {
		var (
			batchCount int
			totalRows  int
		)
		err := s.FindUserInBatches(ctx, 2, func(batch []model.SysUser, batchNo int) error {
			batchCount++
			totalRows += len(batch)
			t.Logf("批次 %d: %d 条", batchNo, len(batch))
			return nil
		})
		if err != nil {
			t.Fatalf("FindUserInBatches 失败: %v", err)
		}
		if batchCount == 0 {
			t.Error("未执行批次回调")
		}
		t.Logf("共 %d 批次, %d 条记录", batchCount, totalRows)
	})
}
