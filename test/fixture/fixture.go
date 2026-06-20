// Package fixture 提供测试数据定义和数据库播种/清理辅助函数。
package fixture

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go_orm_tt/internal/model"

	"gorm.io/gorm"
)

// UserInput 是创建测试用户所需的输入参数。
type UserInput struct {
	Name     string
	Nickname string
	Password string
	TenantID string
	Remark   string
}

// DefaultUserInput 返回一组默认的测试用户数据。
func DefaultUserInput() UserInput {
	return UserInput{
		Name:     "testuser",
		Nickname: "测试用户",
		Password: "password123",
		TenantID: "default",
		Remark:   "test fixture user",
	}
}

// NewUser 根据输入构造一个 SysUser 对象（不含关联 ID）。
func NewUser(seq int, input UserInput) model.SysUser {
	now := time.Now()
	return model.SysUser{
		ID:        int64(10000 + seq),
		Status:    1,
		Name:      fmt.Sprintf("%s_%d", input.Name, seq),
		Nickname:  fmt.Sprintf("%s_%d", input.Nickname, seq),
		Password:  input.Password,
		TenantID:  input.TenantID,
		CreatedBy: 1,
		CreatedAt: now,
		UpdatedBy: 1,
		UpdatedAt: now,
		Remark:    fmt.Sprintf("%s #%d", input.Remark, seq),
	}
}

// CreateUsers 批量插入用户数据并返回切片。
func CreateUsers(t testing.TB, db *gorm.DB, users []model.SysUser) []model.SysUser {
	t.Helper()

	for i := range users {
		if err := db.Create(&users[i]).Error; err != nil {
			t.Fatalf("创建测试用户失败 (seq=%d): %v", i, err)
		}
	}
	return users
}

// TruncateTables 清空指定表（事务安全，不依赖外键顺序）。
func TruncateTables(t testing.TB, db *gorm.DB, tables ...string) {
	t.Helper()

	for _, table := range tables {
		if err := db.Exec(fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", table)).Error; err != nil {
			t.Fatalf("清空表 %s 失败: %v", table, err)
		}
	}
}

// CleanupSysUser 删除所有 id >= 10000 的测试用户（用于事务测试中真实提交后的清理）。
func CleanupSysUser(ctx context.Context, db *gorm.DB) error {
	return db.WithContext(ctx).Where("id >= ?", 10000).Delete(&model.SysUser{}).Error
}

// FindUserByID 按 ID 查找用户（辅助断言用）。
func FindUserByID(ctx context.Context, db *gorm.DB, id int64) (*model.SysUser, error) {
	var user model.SysUser
	err := db.WithContext(ctx).First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}
