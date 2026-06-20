package gormtest

import (
	"context"
	"testing"

	"go_orm_tt/test/dbtest"
)

// TestPreload_BelongsTo 测试 BelongsTo 预加载。
func TestPreload_BelongsTo(t *testing.T) {
	ctx := context.Background()
	db := dbtest.MustTxDB(t)

	if err := db.AutoMigrate(&Company{}, &Employee{}); err != nil {
		t.Fatalf("AutoMigrate 失败: %v", err)
	}

	// 创建数据
	company := &Company{ID: 50200, Name: "Belongs公司"}
	if err := db.WithContext(ctx).Create(company).Error; err != nil {
		t.Fatalf("创建 Company 失败: %v", err)
	}
	employee := &Employee{ID: 50201, Name: "Belongs员工", CompanyID: company.ID}
	if err := db.WithContext(ctx).Create(employee).Error; err != nil {
		t.Fatalf("创建 Employee 失败: %v", err)
	}

	// Preload Company
	var emp Employee
	if err := db.WithContext(ctx).Preload("Company").First(&emp, employee.ID).Error; err != nil {
		t.Fatalf("Preload Company 失败: %v", err)
	}
	if emp.Company == nil {
		t.Fatal("Preload Company 后 Company 为 nil")
	}
	if emp.Company.Name != "Belongs公司" {
		t.Errorf("Company.Name = %q, 期望 %q", emp.Company.Name, "Belongs公司")
	}
}

// TestPreload_HasOne 测试 HasOne 预加载。
func TestPreload_HasOne(t *testing.T) {
	ctx := context.Background()
	db := dbtest.MustTxDB(t)

	if err := db.AutoMigrate(&TestUser{}, &UserProfile{}); err != nil {
		t.Fatalf("AutoMigrate 失败: %v", err)
	}

	// 创建用户
	user := &TestUser{ID: 50300, Name: "hasone_user", Nickname: "HasOne测试", Password: "pass", TenantID: "default"}
	if err := db.WithContext(ctx).Create(user).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}

	// 创建 Profile
	profile := &UserProfile{ID: 50301, UserID: user.ID, Bio: "HasOne 简介"}
	if err := db.WithContext(ctx).Create(profile).Error; err != nil {
		t.Fatalf("创建 Profile 失败: %v", err)
	}

	// Preload Profile
	var u TestUser
	if err := db.WithContext(ctx).Preload("Profile").First(&u, user.ID).Error; err != nil {
		t.Fatalf("Preload Profile 失败: %v", err)
	}
	if u.Profile == nil {
		t.Fatal("Preload Profile 后 Profile 为 nil")
	}
	if u.Profile.Bio != "HasOne 简介" {
		t.Errorf("Profile.Bio = %q, 期望 %q", u.Profile.Bio, "HasOne 简介")
	}
}

// TestPreload_ManyToMany 测试 ManyToMany 预加载。
func TestPreload_ManyToMany(t *testing.T) {
	ctx := context.Background()
	db := dbtest.MustTxDB(t)

	if err := db.AutoMigrate(&TestUser{}, &Project{}, &UserProfile{}); err != nil {
		t.Fatalf("AutoMigrate 失败: %v", err)
	}

	// 创建用户
	user := &TestUser{ID: 50400, Name: "m2m_user", Nickname: "ManyToMany测试", Password: "pass", TenantID: "default"}
	if err := db.WithContext(ctx).Create(user).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}

	// 创建项目
	projects := []*Project{
		{ID: 50401, Name: "项目A"},
		{ID: 50402, Name: "项目B"},
	}
	for _, p := range projects {
		if err := db.WithContext(ctx).Create(p).Error; err != nil {
			t.Fatalf("创建 Project 失败: %v", err)
		}
	}

	// 关联用户和项目
	if err := db.WithContext(ctx).Model(user).Association("Projects").Append(projects[0], projects[1]); err != nil {
		t.Fatalf("关联 Projects 失败: %v", err)
	}

	// Preload Projects
	type testUserWith struct {
		TestUser
		Projects []*Project
	}
	var u TestUser
	if err := db.WithContext(ctx).Preload("Projects").First(&u, user.ID).Error; err != nil {
		t.Fatalf("Preload Projects 失败: %v", err)
	}
	if len(u.Projects) != 2 {
		t.Fatalf("预加载 Projects 数量 = %d, 期望 2", len(u.Projects))
	}
	if u.Projects[0].Name != "项目A" || u.Projects[1].Name != "项目B" {
		t.Errorf("Project 数据不正确: %+v", u.Projects)
	}
}

// TestPreload_Nested 测试嵌套预加载。
func TestPreload_Nested(t *testing.T) {
	ctx := context.Background()
	db := dbtest.MustTxDB(t)

	if err := db.AutoMigrate(&Company{}, &Employee{}); err != nil {
		t.Fatalf("AutoMigrate 失败: %v", err)
	}

	company := &Company{ID: 50500, Name: "嵌套公司"}
	if err := db.WithContext(ctx).Create(company).Error; err != nil {
		t.Fatalf("创建 Company 失败: %v", err)
	}
	employee := &Employee{ID: 50501, Name: "嵌套员工", CompanyID: company.ID}
	if err := db.WithContext(ctx).Create(employee).Error; err != nil {
		t.Fatalf("创建 Employee 失败: %v", err)
	}

	// 嵌套预加载 Company（如果 Employee 有更多关联，可以链式 Preload）
	var emp Employee
	if err := db.WithContext(ctx).
		Preload("Company").
		First(&emp, employee.ID).Error; err != nil {
		t.Fatalf("嵌套 Preload 失败: %v", err)
	}
	if emp.Company == nil {
		t.Fatal("嵌套 Preload 后 Company 为 nil")
	}
}

// TestPreload_Condition 测试带条件的预加载。
func TestPreload_Condition(t *testing.T) {
	ctx := context.Background()
	db := dbtest.MustTxDB(t)

	if err := db.AutoMigrate(&TestUser{}, &Project{}, &UserProfile{}); err != nil {
		t.Fatalf("AutoMigrate 失败: %v", err)
	}

	user := &TestUser{ID: 50600, Name: "cond_user", Nickname: "条件预加载", Password: "pass", TenantID: "default"}
	if err := db.WithContext(ctx).Create(user).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}

	projects := []*Project{
		{ID: 50601, Name: "激活项目"},
		{ID: 50602, Name: "已归档项目"},
	}
	for _, p := range projects {
		if err := db.WithContext(ctx).Create(p).Error; err != nil {
			t.Fatalf("创建 Project 失败: %v", err)
		}
	}
	if err := db.WithContext(ctx).Model(user).Association("Projects").Append(projects[0], projects[1]); err != nil {
		t.Fatalf("关联 Projects 失败: %v", err)
	}

	// 带条件的 Preload：只加载名为 "激活项目" 的项目
	var u TestUser
	if err := db.WithContext(ctx).
		Preload("Projects", "name = ?", "激活项目").
		First(&u, user.ID).Error; err != nil {
		t.Fatalf("条件 Preload 失败: %v", err)
	}
	if len(u.Projects) != 1 {
		t.Fatalf("条件 Preload 后 Projects 数量 = %d, 期望 1", len(u.Projects))
	}
	if u.Projects[0].Name != "激活项目" {
		t.Errorf("Project.Name = %q, 期望 %q", u.Projects[0].Name, "激活项目")
	}
}

// TestPreload_WithQuery 测试预加载 + 条件查询组合。
func TestPreload_WithQuery(t *testing.T) {
	ctx := context.Background()
	db := dbtest.MustTxDB(t)

	if err := db.AutoMigrate(&TestUser{}, &Project{}, &UserProfile{}); err != nil {
		t.Fatalf("AutoMigrate 失败: %v", err)
	}

	// 创建多个用户
	for i, name := range []string{"user_a", "user_b"} {
		u := &TestUser{
			ID:       int64(50700 + i),
			Name:     name,
			Nickname: "条件查询",
			Password: "pass",
			TenantID: "default",
		}
		if err := db.WithContext(ctx).Create(u).Error; err != nil {
			t.Fatalf("创建用户 %s 失败: %v", name, err)
		}

		// 给 user_a 关联一个项目
		if i == 0 {
			p := &Project{ID: 50710, Name: "用户A的项目"}
			if err := db.WithContext(ctx).Create(p).Error; err != nil {
				t.Fatalf("创建 Project 失败: %v", err)
			}
			if err := db.WithContext(ctx).Model(u).Association("Projects").Append(p); err != nil {
				t.Fatalf("关联 Projects 失败: %v", err)
			}
		}
	}

	// 查询 user_a 并预加载 Projects
	var u TestUser
	if err := db.WithContext(ctx).
		Preload("Projects").
		Where("name = ?", "user_a").
		First(&u).Error; err != nil {
		t.Fatalf("查询+预加载失败: %v", err)
	}
	if len(u.Projects) != 1 {
		t.Errorf("Projects 数量 = %d, 期望 1", len(u.Projects))
	}
}
