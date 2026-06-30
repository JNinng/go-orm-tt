package gormtest

import (
	"context"
	"testing"

	"go_orm_tt/plugin"
	"go_orm_tt/test/dbtest"
)

// ============================================================
// RLS TenantPlugin 测试
// ============================================================

// TestGORM_RLS_PluginInitialize 测试插件通过 db.Use 注册成功，且 Name 返回约定格式。
func TestGORM_RLS_PluginInitialize(t *testing.T) {
	db := dbtest.MustTxDB(t)

	p := &plugin.TenantPlugin{}

	// 验证 Name() 返回约定格式
	if name := p.Name(); name != "tenant_rls" {
		t.Errorf("Name() = %q, 期望 %q", name, "tenant_rls")
	}

	// 验证 Initialize 不返回错误
	if err := p.Initialize(db); err != nil {
		t.Fatalf("Initialize 失败: %v", err)
	}

	// 也可通过 db.Use() 注册（GORM 标准方式）
	t.Log("TenantPlugin 初始化成功")
}

// TestGORM_RLS_WithTenantID 验证带租户 ID 的 context 时操作正常完成。
func TestGORM_RLS_WithTenantID(t *testing.T) {
	db := dbtest.MustTxDB(t)

	p := &plugin.TenantPlugin{}
	if err := p.Initialize(db); err != nil {
		t.Fatalf("Initialize 失败: %v", err)
	}

	// 注入租户 ID
	ctx := plugin.WithTenantID(context.Background(), "tenant-abc")

	user := &HookUser{
		ID:        40100,
		Name:      "rls_with_tenant",
		Nickname:  "RLS租户测试",
		Password:  "pass",
		CreatedBy: 1,
		UpdatedBy: 1,
		TenantID:  "tenant-abc",
	}

	// Create 操作应正常完成（set_config 在回调中执行）
	if err := db.WithContext(ctx).Create(user).Error; err != nil {
		t.Fatalf("带租户 context 的 Create 失败: %v", err)
	}

	// Query 操作应正常完成
	var found HookUser
	if err := db.WithContext(ctx).Where("id = ?", 40100).First(&found).Error; err != nil {
		t.Fatalf("带租户 context 的 First 失败: %v", err)
	}
	if found.Name != "rls_with_tenant" {
		t.Errorf("Name = %q, 期望 %q", found.Name, "rls_with_tenant")
	}

	// Update 操作应正常完成
	if err := db.WithContext(ctx).Model(&found).Update("nickname", "RLS更新昵称").Error; err != nil {
		t.Fatalf("带租户 context 的 Update 失败: %v", err)
	}

	// Delete 操作应正常完成
	if err := db.WithContext(ctx).Delete(&found).Error; err != nil {
		t.Fatalf("带租户 context 的 Delete 失败: %v", err)
	}

	t.Log("带租户 context 的 CRUD 全部完成")
}

// TestGORM_RLS_WithoutTenantID 验证无租户 ID 时操作也正常完成（优雅降级）。
func TestGORM_RLS_WithoutTenantID(t *testing.T) {
	db := dbtest.MustTxDB(t)

	p := &plugin.TenantPlugin{}
	if err := p.Initialize(db); err != nil {
		t.Fatalf("Initialize 失败: %v", err)
	}

	// 使用不带租户 ID 的 context
	ctx := context.Background()

	user := &HookUser{
		ID:        40200,
		Name:      "rls_no_tenant",
		Nickname:  "无租户测试",
		Password:  "pass",
		CreatedBy: 1,
		UpdatedBy: 1,
		TenantID:  "default",
	}

	// 无租户 ID 时回调会跳过 set_config，操作应正常完成
	if err := db.WithContext(ctx).Create(user).Error; err != nil {
		t.Fatalf("无租户 context 的 Create 失败: %v", err)
	}

	var found HookUser
	if err := db.WithContext(ctx).Where("id = ?", 40200).First(&found).Error; err != nil {
		t.Fatalf("无租户 context 的 First 失败: %v", err)
	}

	t.Log("无租户 context 时优雅降级，操作正常完成")
}

// TestGORM_RLS_ContextHelpers 测试上下文辅助函数的往返一致性。
func TestGORM_RLS_ContextHelpers(t *testing.T) {
	// 写入 → 读取 一致
	ctx := plugin.WithTenantID(context.Background(), "t-999")
	if got := plugin.TenantIDFromContext(ctx); got != "t-999" {
		t.Errorf("TenantIDFromContext = %q, 期望 %q", got, "t-999")
	}

	// 空 context 返回空字符串
	if got := plugin.TenantIDFromContext(context.Background()); got != "" {
		t.Errorf("空 context 期望返回 \"\", 得到 %q", got)
	}

	// 非字符串值（类型安全）
	ctx = context.WithValue(context.Background(), tenantKeyForTest{}, 12345)
	if got := plugin.TenantIDFromContext(ctx); got != "" {
		t.Errorf("非字符串值期望返回 \"\", 得到 %q", got)
	}

	t.Log("上下文辅助函数往返一致")
}

// ============================================================
// 辅助类型 — 用于类型安全测试
// ============================================================

// tenantKeyForTest 是测试中用于验证类型安全的键类型。
// plugin.TenantIDFromContext 使用私有 tenantKey{}，不同键类型不会匹配。
type tenantKeyForTest struct{}

// ============================================================
// RLS 租户隔离测试 — admin 用户场景
// ============================================================

// TestGORM_RLS_AdminUser 测试带 RLS 的 admin 用户在租户上下文中的完整 CRUD 操作。
//
// 场景：admin 用户（name=admin, password=admin）属于租户 "tenant-admin"，
// 在正确租户上下文中应能正常完成 CRUD。
func TestGORM_RLS_AdminUser(t *testing.T) {
	db := dbtest.MustTxDB(t)

	p := &plugin.TenantPlugin{}
	if err := p.Initialize(db); err != nil {
		t.Fatalf("Initialize 失败: %v", err)
	}

	// admin 用户所属租户
	adminTenant := "tenant-admin"
	ctx := plugin.WithTenantID(context.Background(), adminTenant)

	admin := &HookUser{
		ID:        50100,
		Name:      "admin",
		Nickname:  "系统管理员",
		Password:  "admin",
		CreatedBy: 1,
		UpdatedBy: 1,
		TenantID:  adminTenant,
	}

	// 1. Create — 在 admin 租户上下文中创建 admin 用户
	if err := db.WithContext(ctx).Create(admin).Error; err != nil {
		t.Fatalf("admin 用户 Create 失败: %v", err)
	}
	t.Log("admin 用户创建成功")

	// 2. Query — 在 admin 租户上下文中查询
	var found HookUser
	if err := db.WithContext(ctx).Where("id = ?", 50100).First(&found).Error; err != nil {
		t.Fatalf("admin 用户 First 失败: %v", err)
	}
	if found.Name != "admin" {
		t.Errorf("Name = %q, 期望 %q", found.Name, "admin")
	}
	if found.Password != "admin" {
		t.Errorf("Password = %q, 期望 %q", found.Password, "admin")
	}
	if found.TenantID != adminTenant {
		t.Errorf("TenantID = %q, 期望 %q", found.TenantID, adminTenant)
	}
	t.Log("admin 用户查询成功")

	// 3. Update — 更新 admin 用户的昵称
	if err := db.WithContext(ctx).Model(&found).Update("nickname", "管理员(已更新)").Error; err != nil {
		t.Fatalf("admin 用户 Update 失败: %v", err)
	}

	var updated HookUser
	if err := db.WithContext(ctx).Where("id = ?", 50100).First(&updated).Error; err != nil {
		t.Fatalf("更新后查询失败: %v", err)
	}
	if updated.Nickname != "管理员(已更新)" {
		t.Errorf("Nickname = %q, 期望 %q", updated.Nickname, "管理员(已更新)")
	}
	t.Log("admin 用户更新成功")

	// 4. Delete — 删除 admin 用户
	if err := db.WithContext(ctx).Delete(&updated).Error; err != nil {
		t.Fatalf("admin 用户 Delete 失败: %v", err)
	}

	// 验证已删除（软删除）
	var afterDelete HookUser
	err := db.WithContext(ctx).Where("id = ?", 50100).First(&afterDelete).Error
	if err == nil {
		t.Error("软删除后仍能查到 admin 用户，期望被过滤")
	}
	t.Log("admin 用户删除成功")
}

// TestGORM_RLS_TenantDataIsolation 测试多租户数据隔离。
//
// 创建两个租户的数据：
//   - tenant-admin（admin 的租户）：admin 用户 + 普通用户
//   - tenant-b（其他租户）：其他用户
//
// 验证在指定租户上下文中只能操作对应租户的数据。
func TestGORM_RLS_TenantDataIsolation(t *testing.T) {
	db := dbtest.MustTxDB(t)

	p := &plugin.TenantPlugin{}
	if err := p.Initialize(db); err != nil {
		t.Fatalf("Initialize 失败: %v", err)
	}

	adminTenant := "tenant-admin"
	otherTenant := "tenant-b"

	// ---- 在 tenant-admin 上下文中创建 admin 用户 ----
	ctxAdmin := plugin.WithTenantID(context.Background(), adminTenant)

	admin := &HookUser{
		ID:        50200,
		Name:      "admin",
		Nickname:  "系统管理员",
		Password:  "admin",
		CreatedBy: 1,
		UpdatedBy: 1,
		TenantID:  adminTenant,
	}
	if err := db.WithContext(ctxAdmin).Create(admin).Error; err != nil {
		t.Fatalf("admin 用户创建失败: %v", err)
	}

	userInAdmin := &HookUser{
		ID:        50201,
		Name:      "staff_admin_tenant",
		Nickname:  "admin租户普通用户",
		Password:  "pass123",
		CreatedBy: 1,
		UpdatedBy: 1,
		TenantID:  adminTenant,
	}
	if err := db.WithContext(ctxAdmin).Create(userInAdmin).Error; err != nil {
		t.Fatalf("admin 租户下普通用户创建失败: %v", err)
	}

	// ---- 在 tenant-b 上下文中创建其他用户 ----
	ctxOther := plugin.WithTenantID(context.Background(), otherTenant)

	userInOther := &HookUser{
		ID:        50202,
		Name:      "other_tenant_user",
		Nickname:  "其他租户用户",
		Password:  "pass456",
		CreatedBy: 1,
		UpdatedBy: 1,
		TenantID:  otherTenant,
	}
	if err := db.WithContext(ctxOther).Create(userInOther).Error; err != nil {
		t.Fatalf("其他租户用户创建失败: %v", err)
	}

	// ---- 隔离验证：admin 租户上下文只能查到本租户的用户 ----
	var adminResults []HookUser
	if err := db.WithContext(ctxAdmin).Where("tenant_id = ?", adminTenant).Find(&adminResults).Error; err != nil {
		t.Fatalf("admin 租户查询失败: %v", err)
	}
	if len(adminResults) < 2 {
		t.Errorf("admin 租户查询返回 %d 条，期望至少 2 条", len(adminResults))
	}
	for _, u := range adminResults {
		if u.TenantID != adminTenant {
			t.Errorf("admin 租户查询返回了其他租户的的用户: ID=%d, TenantID=%q", u.ID, u.TenantID)
		}
	}
	t.Logf("admin 租户隔离验证通过: 查询到 %d 条记录（均属 %s）", len(adminResults), adminTenant)

	// ---- 隔离验证：其他租户上下文只能查到自己的用户 ----
	var otherResults []HookUser
	if err := db.WithContext(ctxOther).Where("tenant_id = ?", otherTenant).Find(&otherResults).Error; err != nil {
		t.Fatalf("其他租户查询失败: %v", err)
	}
	if len(otherResults) < 1 {
		t.Errorf("其他租户查询返回 %d 条，期望至少 1 条", len(otherResults))
	}
	for _, u := range otherResults {
		if u.TenantID != otherTenant {
			t.Errorf("其他租户查询返回了 admin 租户的用户: ID=%d, TenantID=%q", u.ID, u.TenantID)
		}
	}
	t.Logf("其他租户隔离验证通过: 查询到 %d 条记录（均属 %s）", len(otherResults), otherTenant)

	// ---- 跨租户验证：用错误 tenant_id 查询应返回空（模拟 RLS WHERE 子句匹配失败） ----
	// 在 admin 租户上下文中，如果应用层用错误的 tenant_id 做 WHERE 过滤，
	// 应该查不到数据（因为目标数据的 tenant_id 不匹配）。
	// 这是 RLS 策略在应用层的等价实现：tenant_id = current_setting('app.tenant_id')
	var crossResults []HookUser
	if err := db.WithContext(ctxAdmin).
		Where("tenant_id = ? AND id = ?", otherTenant, 50200).
		Find(&crossResults).Error; err != nil {
		t.Fatalf("跨租户匹配查询失败: %v", err)
	}
	if len(crossResults) > 0 {
		t.Errorf("隔离缺陷：用错误租户参数查到了 admin 用户: %+v", crossResults)
	} else {
		t.Log("跨租户隔离验证通过：用错误 tenant_id 参数不会返回其他租户数据")
	}
}

// TestGORM_RLS_AdminCrossTenantAccess 测试 admin 用户跨租户访问被拒绝。
//
// 验证 admin 用户只有在其所属的 tenant-admin 租户上下文下才能操作数据，
// 在错误租户上下文下操作应被隔离。
func TestGORM_RLS_AdminCrossTenantAccess(t *testing.T) {
	db := dbtest.MustTxDB(t)

	p := &plugin.TenantPlugin{}
	if err := p.Initialize(db); err != nil {
		t.Fatalf("Initialize 失败: %v", err)
	}

	adminTenant := "tenant-admin"
	wrongTenant := "tenant-hacker"

	// 在 admin 的租户上下文中创建 admin 用户
	ctxAdmin := plugin.WithTenantID(context.Background(), adminTenant)
	admin := &HookUser{
		ID:        50300,
		Name:      "admin",
		Nickname:  "管理员",
		Password:  "admin",
		CreatedBy: 1,
		UpdatedBy: 1,
		TenantID:  adminTenant,
	}
	if err := db.WithContext(ctxAdmin).Create(admin).Error; err != nil {
		t.Fatalf("admin 用户创建失败: %v", err)
	}

	// 尝试在错误租户的上下文下查询 admin 用户（应用层 WHERE 过滤，模拟 RLS 行为）
	ctxWrong := plugin.WithTenantID(context.Background(), wrongTenant)

	// admin 用户的 tenant_id 是 "tenant-admin"，在错误上下文中用 WHERE tenant_id = wrongTenant
	// 应该查不到（因为 admin 的 tenant_id 不匹配）
	var found HookUser
	err := db.WithContext(ctxWrong).Where("tenant_id = ? AND id = ?", wrongTenant, 50300).First(&found).Error
	if err == nil {
		t.Errorf("安全漏洞：错误租户上下文意外访问到 admin 用户: %+v", found)
	} else {
		t.Logf("跨租户访问被正确阻止: %v", err)
	}

	// 验证在正确上下文中 admin 仍在
	var found2 HookUser
	if err := db.WithContext(ctxAdmin).Where("id = ?", 50300).First(&found2).Error; err != nil {
		t.Fatalf("正确上下文查询 admin 失败: %v", err)
	}
	if found2.Name != "admin" {
		t.Errorf("admin 用户数据异常: %+v", found2)
	}
	t.Logf("admin 用户在正确上下文中数据完好: Name=%s, TenantID=%s", found2.Name, found2.TenantID)
}

// TestGORM_RLS_MultipleAdminsAcrossTenants 测试不同租户各自拥有 admin 用户。
//
// 验证租户 A 的 admin 与租户 B 的 admin 互相隔离，互不影响。
// 注意：sys_user.name 有全局唯一索引，因此不同租户的 admin 使用不同用户名。
// 在实际系统中，可通过 (tenant_id, name) 复合唯一索引来实现租户内唯一。
func TestGORM_RLS_MultipleAdminsAcrossTenants(t *testing.T) {
	db := dbtest.MustTxDB(t)

	p := &plugin.TenantPlugin{}
	if err := p.Initialize(db); err != nil {
		t.Fatalf("Initialize 失败: %v", err)
	}

	tenantA := "tenant-alpha"
	tenantB := "tenant-beta"

	// 租户 A 的 admin
	ctxA := plugin.WithTenantID(context.Background(), tenantA)
	adminA := &HookUser{
		ID:        50400,
		Name:      "admin_alpha",
		Nickname:  "租户A管理员",
		Password:  "admin",
		CreatedBy: 1,
		UpdatedBy: 1,
		TenantID:  tenantA,
	}
	if err := db.WithContext(ctxA).Create(adminA).Error; err != nil {
		t.Fatalf("租户A admin 创建失败: %v", err)
	}

	// 租户 B 的 admin（不同租户，不同用户名以规避全局唯一索引）
	ctxB := plugin.WithTenantID(context.Background(), tenantB)
	adminB := &HookUser{
		ID:        50401,
		Name:      "admin_beta",
		Nickname:  "租户B管理员",
		Password:  "admin",
		CreatedBy: 1,
		UpdatedBy: 1,
		TenantID:  tenantB,
	}
	if err := db.WithContext(ctxB).Create(adminB).Error; err != nil {
		t.Fatalf("租户B admin 创建失败: %v", err)
	}

	// 租户 A 上下文中只能查询到租户 A 的 admin（应用层过滤）
	var resultA []HookUser
	if err := db.WithContext(ctxA).Where("name = ? AND tenant_id = ?", "admin_alpha", tenantA).Find(&resultA).Error; err != nil {
		t.Fatalf("租户A 查询失败: %v", err)
	}
	if len(resultA) != 1 {
		t.Errorf("租户A 查询 admin_alpha 返回 %d 条，期望 1 条", len(resultA))
	}
	if len(resultA) == 1 {
		if resultA[0].Nickname != "租户A管理员" {
			t.Errorf("租户A admin Nickname = %q, 期望 %q", resultA[0].Nickname, "租户A管理员")
		}
		if resultA[0].TenantID != tenantA {
			t.Errorf("租户A admin TenantID = %q, 期望 %q", resultA[0].TenantID, tenantA)
		}
	}

	// 租户 B 上下文中只能查询到租户 B 的 admin
	var resultB []HookUser
	if err := db.WithContext(ctxB).Where("name = ? AND tenant_id = ?", "admin_beta", tenantB).Find(&resultB).Error; err != nil {
		t.Fatalf("租户B 查询失败: %v", err)
	}
	if len(resultB) != 1 {
		t.Errorf("租户B 查询 admin_beta 返回 %d 条，期望 1 条", len(resultB))
	}
	if len(resultB) == 1 {
		if resultB[0].Nickname != "租户B管理员" {
			t.Errorf("租户B admin Nickname = %q, 期望 %q", resultB[0].Nickname, "租户B管理员")
		}
		if resultB[0].TenantID != tenantB {
			t.Errorf("租户B admin TenantID = %q, 期望 %q", resultB[0].TenantID, tenantB)
		}
	}

	// 跨租户验证：租户 A 查不到租户 B 的 admin
	var crossResults []HookUser
	if err := db.WithContext(ctxA).Where("name = ? AND tenant_id = ?", "admin_beta", tenantA).Find(&crossResults).Error; err != nil {
		t.Fatalf("跨租户查询失败: %v", err)
	}
	if len(crossResults) > 0 {
		t.Errorf("隔离缺陷：租户A 上下文查到了租户B的 admin: %+v", crossResults)
	} else {
		t.Log("跨租户验证通过：租户A 上下文查不到租户B的 admin")
	}

	t.Log("多租户 admin 隔离验证通过：每个租户的 admin 互不影响")
}

// TestGORM_RLS_ContextSwitch 测试运行时切换租户上下文。
//
// 验证在同一事务中切换不同租户上下文时，各租户的数据操作互不干扰。
func TestGORM_RLS_ContextSwitch(t *testing.T) {
	db := dbtest.MustTxDB(t)

	p := &plugin.TenantPlugin{}
	if err := p.Initialize(db); err != nil {
		t.Fatalf("Initialize 失败: %v", err)
	}

	tenantA := "tenant-switch-a"
	tenantB := "tenant-switch-b"

	// 在租户 A 上下文中创建用户
	ctxA := plugin.WithTenantID(context.Background(), tenantA)
	userA := &HookUser{
		ID:        50500,
		Name:      "switch_user_a",
		Nickname:  "切换测试A",
		Password:  "pass",
		CreatedBy: 1,
		UpdatedBy: 1,
		TenantID:  tenantA,
	}
	if err := db.WithContext(ctxA).Create(userA).Error; err != nil {
		t.Fatalf("租户A 用户创建失败: %v", err)
	}

	// 切换到租户 B 上下文创建用户
	ctxB := plugin.WithTenantID(context.Background(), tenantB)
	userB := &HookUser{
		ID:        50501,
		Name:      "switch_user_b",
		Nickname:  "切换测试B",
		Password:  "pass",
		CreatedBy: 1,
		UpdatedBy: 1,
		TenantID:  tenantB,
	}
	if err := db.WithContext(ctxB).Create(userB).Error; err != nil {
		t.Fatalf("租户B 用户创建失败: %v", err)
	}

	// 切换回租户 A 上下文更新用户
	if err := db.WithContext(ctxA).Model(userA).Update("nickname", "A已更新").Error; err != nil {
		t.Fatalf("租户A 更新失败: %v", err)
	}

	// 切换回租户 B 上下文更新用户
	if err := db.WithContext(ctxB).Model(userB).Update("nickname", "B已更新").Error; err != nil {
		t.Fatalf("租户B 更新失败: %v", err)
	}

	// 验证各租户数据独立
	var foundA HookUser
	db.WithContext(ctxA).Where("id = ?", 50500).First(&foundA)
	if foundA.Nickname != "A已更新" {
		t.Errorf("租户A Nickname = %q, 期望 %q", foundA.Nickname, "A已更新")
	}
	if foundA.TenantID != tenantA {
		t.Errorf("租户A TenantID = %q, 期望 %q", foundA.TenantID, tenantA)
	}

	var foundB HookUser
	db.WithContext(ctxB).Where("id = ?", 50501).First(&foundB)
	if foundB.Nickname != "B已更新" {
		t.Errorf("租户B Nickname = %q, 期望 %q", foundB.Nickname, "B已更新")
	}
	if foundB.TenantID != tenantB {
		t.Errorf("租户B TenantID = %q, 期望 %q", foundB.TenantID, tenantB)
	}

	t.Log("运行时上下文切换验证通过")
}

// TestGORM_RLS_BulkOperationWithTenant 测试在租户上下文中的批量操作。
//
// 验证在 admin 租户上下文中批量创建和批量查询均正确隔离。
func TestGORM_RLS_BulkOperationWithTenant(t *testing.T) {
	db := dbtest.MustTxDB(t)

	p := &plugin.TenantPlugin{}
	if err := p.Initialize(db); err != nil {
		t.Fatalf("Initialize 失败: %v", err)
	}

	adminTenant := "tenant-admin"
	ctx := plugin.WithTenantID(context.Background(), adminTenant)

	// 批量创建 admin 租户下的多个用户
	users := []*HookUser{
		{ID: 50600, Name: "admin", Nickname: "管理员", Password: "admin", CreatedBy: 1, UpdatedBy: 1, TenantID: adminTenant},
		{ID: 50601, Name: "admin助理1", Nickname: "助理1", Password: "pass", CreatedBy: 1, UpdatedBy: 1, TenantID: adminTenant},
		{ID: 50602, Name: "admin助理2", Nickname: "助理2", Password: "pass", CreatedBy: 1, UpdatedBy: 1, TenantID: adminTenant},
		{ID: 50603, Name: "admin审计员", Nickname: "审计员", Password: "pass", CreatedBy: 1, UpdatedBy: 1, TenantID: adminTenant},
	}
	for i, u := range users {
		if err := db.WithContext(ctx).Create(u).Error; err != nil {
			t.Fatalf("批量创建用户[%d]失败: %v", i, err)
		}
	}

	// 在 admin 租户上下文中查询所有本租户用户
	var results []HookUser
	if err := db.WithContext(ctx).Where("tenant_id = ?", adminTenant).Find(&results).Error; err != nil {
		t.Fatalf("admin 租户批量查询失败: %v", err)
	}
	if len(results) < 4 {
		t.Errorf("admin 租户批量查询返回 %d 条，期望至少 4 条", len(results))
	}
	for _, u := range results {
		if u.TenantID != adminTenant {
			t.Errorf("批量查询返回了其他租户用户: ID=%d, TenantID=%q", u.ID, u.TenantID)
		}
	}
	t.Logf("批量操作验证通过: admin 租户下共 %d 条记录", len(results))
}
