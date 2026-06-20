package gormtest

import (
	"context"
	"fmt"
	"testing"

	"go_orm_tt/internal/model"
	"go_orm_tt/internal/query"
	"go_orm_tt/test/dbtest"
	"go_orm_tt/test/suite"
)

// TestGORM_CRUD 运行 GORM CRUD 标准测试套件。
func TestGORM_CRUD(t *testing.T) {
	db := dbtest.MustTxDB(t)
	q := query.Use(db)

	s := NewGORMSuite(db, q)
	ctx := context.Background()

	suite.RunUserCRUDTests(t, s, ctx)
}

// TestGORM_Finder 运行 GORM 批量查询标准测试套件。
func TestGORM_Finder(t *testing.T) {
	db := dbtest.MustTxDB(t)
	q := query.Use(db)
	ctx := context.Background()

	// 准备测试数据
	for i := 0; i < 5; i++ {
		id := int64(20000 + i)
		user := &model.SysUser{
			ID:       id,
			Name:     fmt.Sprintf("finder_%d", i),
			Nickname: "Finder测试",
			Password: "pass",
			TenantID: "default",
		}
		if err := q.SysUser.WithContext(ctx).Create(user); err != nil {
			t.Fatalf("准备测试数据失败: %v", err)
		}
	}

	s := NewGORMSuite(db, q)

	suite.RunUserFinderTests(t, s, ctx)
}
