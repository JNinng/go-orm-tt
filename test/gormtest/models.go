package gormtest

import (
	"time"

	"gorm.io/gorm"
)

// ============================================================
// HookUser — 用于测试 GORM 钩子的模型
// ============================================================

// HookUser 是一个带 GORM 钩子的测试用用户模型。
// 每次 BeforeCreate 时自动设置 CreatedAt，并在 AuditLog 中记录操作。
type HookUser struct {
	ID        int64  `gorm:"primaryKey"`
	Name      string `gorm:"column:name"`
	Nickname  string `gorm:"column:nickname"`
	Password  string `gorm:"column:password"`
	TenantID  string `gorm:"column:tenant_id"`
	Remark    string `gorm:"column:remark"`
	CreatedBy int64  `gorm:"column:created_by"`
	UpdatedBy int64  `gorm:"column:updated_by"`

	CreatedAt time.Time      `gorm:"column:created_at"`
	UpdatedAt time.Time      `gorm:"column:updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index"`

	// HookContext 用于在测试中验证钩子是否执行
	HookContext *HookExecContext `gorm:"-:all"`
}

func (HookUser) TableName() string { return "sys_user" }

// BeforeCreate 钩子：自动设置 CreatedAt，并记录钩子执行。
func (u *HookUser) BeforeCreate(tx *gorm.DB) error {
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now()
	}
	if u.HookContext != nil {
		u.HookContext.Record("BeforeCreate", u.ID)
	}
	return nil
}

// AfterCreate 钩子：记录钩子执行。
func (u *HookUser) AfterCreate(tx *gorm.DB) error {
	if u.HookContext != nil {
		u.HookContext.Record("AfterCreate", u.ID)
	}
	return nil
}

// BeforeUpdate 钩子：自动设置 UpdatedAt，并记录钩子执行。
func (u *HookUser) BeforeUpdate(tx *gorm.DB) error {
	u.UpdatedAt = time.Now()
	if u.HookContext != nil {
		u.HookContext.Record("BeforeUpdate", u.ID)
	}
	return nil
}

// AfterUpdate 钩子：记录钩子执行。
func (u *HookUser) AfterUpdate(tx *gorm.DB) error {
	if u.HookContext != nil {
		u.HookContext.Record("AfterUpdate", u.ID)
	}
	return nil
}

// BeforeDelete 钩子：记录钩子执行。
func (u *HookUser) BeforeDelete(tx *gorm.DB) error {
	if u.HookContext != nil {
		u.HookContext.Record("BeforeDelete", u.ID)
	}
	return nil
}

// AfterDelete 钩子：记录钩子执行。
func (u *HookUser) AfterDelete(tx *gorm.DB) error {
	if u.HookContext != nil {
		u.HookContext.Record("AfterDelete", u.ID)
	}
	return nil
}

// AfterFind 钩子：记录钩子执行。
func (u *HookUser) AfterFind(tx *gorm.DB) error {
	if u.HookContext != nil {
		u.HookContext.Record("AfterFind", u.ID)
	}
	return nil
}

// ============================================================
// HookExecContext — 钩子执行记录器
// ============================================================

// HookExecContext 记录钩子执行情况，用于测试断言。
type HookExecContext struct {
	Records []HookRecord
}

// HookRecord 单条钩子执行记录。
type HookRecord struct {
	Hook   string
	UserID int64
}

// Record 添加一条执行记录。
func (c *HookExecContext) Record(hook string, userID int64) {
	c.Records = append(c.Records, HookRecord{Hook: hook, UserID: userID})
}

// Contains 检查是否包含指定钩子的执行记录。
func (c *HookExecContext) Contains(hook string) bool {
	for _, r := range c.Records {
		if r.Hook == hook {
			return true
		}
	}
	return false
}

// Count 返回指定钩子名称的执行次数。
func (c *HookExecContext) Count(hook string) int {
	n := 0
	for _, r := range c.Records {
		if r.Hook == hook {
			n++
		}
	}
	return n
}

// ============================================================
// 预加载测试模型
// ============================================================

// Company 公司表（用于 BelongsTo 测试）。
type Company struct {
	ID   int64  `gorm:"primaryKey"`
	Name string `gorm:"column:name"`
}

func (Company) TableName() string { return "test_company" }

// Employee 员工表（用于 HasMany / BelongsTo 测试）。
type Employee struct {
	ID        int64  `gorm:"primaryKey"`
	Name      string `gorm:"column:name"`
	CompanyID int64  `gorm:"column:company_id"`

	// 关联
	Company *Company `gorm:"foreignKey:CompanyID"`
}

func (Employee) TableName() string { return "test_employee" }

// UserProfile 用户档案（用于 HasOne 测试）。
type UserProfile struct {
	ID     int64  `gorm:"primaryKey"`
	UserID int64  `gorm:"column:user_id;uniqueIndex"`
	Bio    string `gorm:"column:bio"`
}

func (UserProfile) TableName() string { return "test_user_profile" }

// ErrHookForcedFail 是钩子测试中用于模拟钩子失败的哨兵错误。
var ErrHookForcedFail = &HookError{Msg: "forced hook failure for test"}

// HookError 用于在钩子测试中区分预期错误。
type HookError struct{ Msg string }

func (e *HookError) Error() string { return e.Msg }

// Project 项目表（用于 ManyToMany 测试）。
type Project struct {
	ID      int64       `gorm:"primaryKey"`
	Name    string      `gorm:"column:name"`
	Members []*TestUser `gorm:"many2many:test_user_projects;"`
}

func (Project) TableName() string { return "test_project" }

// TestUser 完整用户模型（用于预加载测试）。
type TestUser struct {
	ID       int64  `gorm:"primaryKey"`
	Name     string `gorm:"column:name"`
	Nickname string `gorm:"column:nickname"`
	Password string `gorm:"column:password"`
	TenantID string `gorm:"column:tenant_id"`

	// 关联
	Profile  *UserProfile `gorm:"foreignKey:UserID"`
	Projects []*Project   `gorm:"many2many:test_user_projects;"`
}

func (TestUser) TableName() string { return "test_user" }
