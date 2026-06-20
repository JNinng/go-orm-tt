// Package gormtest 提供 GORM 对 suite 接口的实现。
package gormtest

import (
	"context"
	"errors"

	"go_orm_tt/internal/model"
	"go_orm_tt/internal/query"

	"gorm.io/gorm"
)

// GORMSuite 实现 suite.UserCRUD 和 suite.UserFinder 接口。
type GORMSuite struct {
	db *gorm.DB
	q  *query.Query
}

// NewGORMSuite 创建 GORM 测试套件实例。
func NewGORMSuite(db *gorm.DB, q *query.Query) *GORMSuite {
	return &GORMSuite{db: db, q: q}
}

// GetUser 按 ID 获取用户（含软删除过滤），不存在时返回 (nil, nil)。
func (s *GORMSuite) GetUser(ctx context.Context, id int64) (*model.SysUser, error) {
	u, err := s.q.SysUser.WithContext(ctx).Where(s.q.SysUser.ID.Eq(id)).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return u, nil
}

// GetUserUnscoped 按 ID 获取用户（忽略软删除过滤），用于验证软删除标记。
func (s *GORMSuite) GetUserUnscoped(ctx context.Context, id int64) (*model.SysUser, error) {
	u, err := s.q.SysUser.WithContext(ctx).Unscoped().Where(s.q.SysUser.ID.Eq(id)).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return u, nil
}

// CreateUser 创建用户并返回创建后的完整记录。
func (s *GORMSuite) CreateUser(ctx context.Context, user *model.SysUser) (*model.SysUser, error) {
	err := s.q.SysUser.WithContext(ctx).Create(user)
	if err != nil {
		return nil, err
	}
	return s.GetUser(ctx, user.ID)
}

// UpdateUser 更新用户，仅更新非零字段。
func (s *GORMSuite) UpdateUser(ctx context.Context, user *model.SysUser) error {
	_, err := s.q.SysUser.WithContext(ctx).
		Where(s.q.SysUser.ID.Eq(user.ID)).
		Updates(user)
	return err
}

// DeleteUser 软删除用户。
func (s *GORMSuite) DeleteUser(ctx context.Context, id int64) error {
	_, err := s.q.SysUser.WithContext(ctx).
		Where(s.q.SysUser.ID.Eq(id)).
		Delete()
	return err
}

// ListUser 分页查询用户。
func (s *GORMSuite) ListUser(ctx context.Context, limit, offset int) ([]*model.SysUser, error) {
	users, _, err := s.q.SysUser.WithContext(ctx).FindByPage(offset, limit)
	return users, err
}

// BatchCreateUser 批量创建用户，返回影响行数。
func (s *GORMSuite) BatchCreateUser(ctx context.Context, users []*model.SysUser) (int64, error) {
	for _, u := range users {
		if err := s.q.SysUser.WithContext(ctx).Create(u); err != nil {
			return 0, err
		}
	}
	return int64(len(users)), nil
}

// FindUser 查询所有用户。
func (s *GORMSuite) FindUser(ctx context.Context) ([]*model.SysUser, error) {
	return s.q.SysUser.WithContext(ctx).Find()
}

// FindUserInBatches 分批查询用户。
// 使用 raw GORM 来避免依赖 gen.Dao 回调签名。
func (s *GORMSuite) FindUserInBatches(ctx context.Context, batchSize int, fn func([]model.SysUser, int) error) error {
	var result []model.SysUser
	db := s.db.WithContext(ctx).
		Model(&model.SysUser{}).
		FindInBatches(&result, batchSize, func(tx *gorm.DB, batch int) error {
			return fn(result, batch)
		})
	return db.Error
}
