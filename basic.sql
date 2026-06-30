-- public.basic 定义

-- Drop table

-- DROP TABLE public.basic;

CREATE TABLE public.basic
(
    id                  int8                                                   NOT NULL,
    status              int2         DEFAULT 1                                 NOT NULL,
    created_by          int8                                                   NOT NULL,
    created_at          timestamp(0) DEFAULT CURRENT_TIMESTAMP                 NOT NULL,
    updated_by          int8                                                   NOT NULL,
    updated_at          timestamp(0) DEFAULT CURRENT_TIMESTAMP                 NOT NULL,
    deleted_at          timestamp(0) DEFAULT NULL::timestamp without time zone NULL,
    tenant_id           varchar(16)                                            NOT NULL,
    owner_user_id       int8                                                   NULL,
    owner_dept_id       int8                                                   NULL,
    owner_user_group_id int8                                                   NULL,
    CONSTRAINT sys_basic_pkey PRIMARY KEY (id)
);
COMMENT ON TABLE public.basic IS '基础字段示例表';

-- Column comments

COMMENT ON COLUMN public.basic.id IS '主键 ID';
COMMENT ON COLUMN public.basic.status IS '数据状态，0：禁用、1：启用';
COMMENT ON COLUMN public.basic.created_by IS '创建人';
COMMENT ON COLUMN public.basic.created_at IS '创建时间';
COMMENT ON COLUMN public.basic.updated_by IS '更新人';
COMMENT ON COLUMN public.basic.updated_at IS '更新时间';
COMMENT ON COLUMN public.basic.deleted_at IS '删除时间';
COMMENT ON COLUMN public.basic.tenant_id IS '租户 ID';
COMMENT ON COLUMN public.basic.owner_user_id IS '归属用户 ID';
COMMENT ON COLUMN public.basic.owner_dept_id IS '归属部门 ID';
COMMENT ON COLUMN public.basic.owner_user_group_id IS '归属用户组 ID';


-- public.sys_dept 定义

-- Drop table

-- DROP TABLE public.sys_dept;

CREATE TABLE public.sys_dept
(
    id                  int8                                                   NOT NULL,
    status              int2         DEFAULT 1                                 NOT NULL,
    created_by          int8                                                   NOT NULL,
    created_at          timestamp(0) DEFAULT CURRENT_TIMESTAMP                 NOT NULL,
    updated_by          int8                                                   NOT NULL,
    updated_at          timestamp(0) DEFAULT CURRENT_TIMESTAMP                 NOT NULL,
    deleted_at          timestamp(0) DEFAULT NULL::timestamp without time zone NULL,
    tenant_id           varchar(16)                                            NOT NULL,
    parent_id           int8                                                   NULL,
    "name"              varchar(256)                                           NOT NULL,
    owner_user_id       int8                                                   NOT NULL,
    owner_user_group_id int8                                                   NULL,
    CONSTRAINT sys_dept_pkey PRIMARY KEY (id)
);
COMMENT ON TABLE public.sys_dept IS '部门表';

-- Column comments

COMMENT ON COLUMN public.sys_dept.id IS '部门 ID';
COMMENT ON COLUMN public.sys_dept.status IS '数据状态，0：禁用、1：启用';
COMMENT ON COLUMN public.sys_dept.created_by IS '创建人';
COMMENT ON COLUMN public.sys_dept.created_at IS '创建时间';
COMMENT ON COLUMN public.sys_dept.updated_by IS '更新人';
COMMENT ON COLUMN public.sys_dept.updated_at IS '更新人';
COMMENT ON COLUMN public.sys_dept.deleted_at IS '删除时间';
COMMENT ON COLUMN public.sys_dept.tenant_id IS '租户 ID';
COMMENT ON COLUMN public.sys_dept.parent_id IS '父级部门 ID';
COMMENT ON COLUMN public.sys_dept."name" IS '部门名';
COMMENT ON COLUMN public.sys_dept.owner_user_id IS '部门管理员用户';
COMMENT ON COLUMN public.sys_dept.owner_user_group_id IS '归属用户组 ID';

-- DROP FUNCTION public.fn_maintain_dept_closure();

CREATE OR REPLACE FUNCTION public.fn_maintain_dept_closure()
    RETURNS trigger
    LANGUAGE plpgsql
AS
$function$
DECLARE
    v_old_parent_id INT8;
    v_new_parent_id INT8;
BEGIN
    -- ================= 1. 新增部门 =================
    IF (TG_OP = 'INSERT') THEN
        -- 插入自身到自身的路径 (深度0)
        INSERT INTO sys_dept_closure (ancestor_id, descendant_id, depth)
        VALUES (NEW.id, NEW.id, 0);

        -- 如果有父节点，复制父节点的所有祖先关系，并指向新节点（深度+1）
        IF NEW.parent_id IS NOT NULL THEN
            INSERT INTO sys_dept_closure (ancestor_id, descendant_id, depth)
            SELECT c.ancestor_id, NEW.id, c.depth + 1
            FROM sys_dept_closure c
            WHERE c.descendant_id = NEW.parent_id;
        END IF;

        RETURN NEW;
    END IF;

    -- ================= 2. 删除部门 =================
    IF (TG_OP = 'DELETE') THEN
        -- 删除与该节点及其所有子节点相关的所有闭包记录
        -- (因为子节点失去了这个祖先)
        DELETE
        FROM sys_dept_closure
        WHERE descendant_id IN (SELECT descendant_id
                                FROM sys_dept_closure
                                WHERE ancestor_id = OLD.id);
        RETURN OLD;
    END IF;

    -- ================= 3. 移动部门 (修改 parent_id) =================
    IF (TG_OP = 'UPDATE' AND NEW.parent_id IS DISTINCT FROM OLD.parent_id) THEN
        -- 步骤 A: 断开旧关系
        -- 删除 "旧祖先 -> 该节点及其子节点" 的路径
        DELETE
        FROM sys_dept_closure
        WHERE descendant_id IN (SELECT descendant_id FROM sys_dept_closure WHERE ancestor_id = OLD.id) -- 后代
          AND ancestor_id IN
              (SELECT ancestor_id FROM sys_dept_closure WHERE descendant_id = OLD.id AND ancestor_id != OLD.id);
        -- 旧祖先(排除自身)

        -- 步骤 B: 建立新关系
        -- 插入 "新祖先 -> 该节点及其子节点" 的路径
        IF NEW.parent_id IS NOT NULL THEN
            INSERT INTO sys_dept_closure (ancestor_id, descendant_id, depth)
            SELECT a.ancestor_id, d.descendant_id, a.depth + d.depth + 1
            FROM sys_dept_closure a -- 新的祖先路径
                     CROSS JOIN sys_dept_closure d -- 当前节点及其子节点的路径
            WHERE a.descendant_id = NEW.parent_id -- 新祖先路径的终点是新父节点
              AND d.ancestor_id = NEW.id          -- 子路径的起点是当前节点
              AND a.ancestor_id != d.descendant_id; -- 避免产生环（自己不能是自己的祖先）
        END IF;

        RETURN NEW;
    END IF;

    RETURN COALESCE(NEW, OLD);
END;
$function$
;

-- Table Triggers

create trigger trg_maintain_dept_closure
    after
        insert
        or
        delete
        or
        update
            of parent_id
    on
        public.sys_dept
    for each row
execute function fn_maintain_dept_closure();


-- public.sys_dept_closure 定义

-- Drop table

-- DROP TABLE public.sys_dept_closure;

CREATE TABLE public.sys_dept_closure
(
    id            bigserial NOT NULL,
    ancestor_id   int8      NOT NULL,
    descendant_id int8      NOT NULL,
    "depth"       int4      NOT NULL,
    CONSTRAINT sys_dept_closure_pkey PRIMARY KEY (ancestor_id, descendant_id)
);
CREATE INDEX idx_closure_ancestor ON public.sys_dept_closure USING btree (ancestor_id);
CREATE INDEX idx_closure_descendant ON public.sys_dept_closure USING btree (descendant_id);
COMMENT ON TABLE public.sys_dept_closure IS '部门闭包表';

-- Column comments

COMMENT ON COLUMN public.sys_dept_closure.ancestor_id IS '祖先节点ID';
COMMENT ON COLUMN public.sys_dept_closure.descendant_id IS '后代节点ID';
COMMENT ON COLUMN public.sys_dept_closure."depth" IS '深度/距离（0表示自身，1表示直接子节点，2表示孙子节点...）';


-- public.sys_role 定义

-- Drop table

-- DROP TABLE public.sys_role;

CREATE TABLE public.sys_role
(
    id            int8                                                   NOT NULL,
    status        int2         DEFAULT 1                                 NOT NULL,
    created_by    int8                                                   NOT NULL,
    created_at    timestamp(0) DEFAULT CURRENT_TIMESTAMP                 NOT NULL,
    updated_by    int8                                                   NOT NULL,
    updated_at    timestamp(0) DEFAULT CURRENT_TIMESTAMP                 NOT NULL,
    deleted_at    timestamp(0) DEFAULT NULL::timestamp without time zone NULL,
    tenant_id     varchar(16)                                            NOT NULL,
    "name"        varchar(256)                                           NOT NULL,
    code          varchar(64)                                            NOT NULL,
    role_type     int2         DEFAULT 2                                 NOT NULL,
    data_scope    int2         DEFAULT 1                                 NOT NULL,
    scope_dept_id int8                                                   NULL,
    remark        varchar(512)                                           NULL,
    CONSTRAINT sys_role_pkey PRIMARY KEY (id),
    CONSTRAINT sys_role_tenant_code_un UNIQUE (tenant_id, code),
    CONSTRAINT sys_role_tenant_name_un UNIQUE (tenant_id, name)
);
COMMENT ON TABLE public.sys_role IS '角色表';

-- Column comments

COMMENT ON COLUMN public.sys_role.id IS '主键 ID';
COMMENT ON COLUMN public.sys_role.status IS '数据状态，0：禁用、1：启用';
COMMENT ON COLUMN public.sys_role.created_by IS '创建人';
COMMENT ON COLUMN public.sys_role.created_at IS '创建时间';
COMMENT ON COLUMN public.sys_role.updated_by IS '更新人';
COMMENT ON COLUMN public.sys_role.updated_at IS '更新时间';
COMMENT ON COLUMN public.sys_role.deleted_at IS '删除时间';
COMMENT ON COLUMN public.sys_role.tenant_id IS '租户 ID';
COMMENT ON COLUMN public.sys_role."name" IS '角色名';
COMMENT ON COLUMN public.sys_role.code IS '权限编码';
COMMENT ON COLUMN public.sys_role.role_type IS '角色类型：1-系统预置(不可删除，核心权限不可剥离) 2-租户自定义';
COMMENT ON COLUMN public.sys_role.data_scope IS '数据范围规则：1-仅本人 2-本部门 3-本部门及子部门 4-指定人 5-指定部门 6-全租户 7-用户组';
COMMENT ON COLUMN public.sys_role.scope_dept_id IS '组织可见范围：限定角色可分配的部门层级。NULL=全租户可分配，具体部门ID=仅该部门及子部门可分配';
COMMENT ON COLUMN public.sys_role.remark IS '备注';


-- public.sys_role_id_scope 定义

-- Drop table

-- DROP TABLE public.sys_role_id_scope;

CREATE TABLE public.sys_role_id_scope
(
    id         int8                                                   NOT NULL,
    status     int2         DEFAULT 1                                 NOT NULL,
    "type"     int2         DEFAULT 1                                 NOT NULL,
    role_id    int8                                                   NOT NULL,
    data_id    int8                                                   NOT NULL,
    created_by int8                                                   NOT NULL,
    created_at timestamp(0) DEFAULT CURRENT_TIMESTAMP                 NOT NULL,
    updated_by int8                                                   NOT NULL,
    updated_at timestamp(0) DEFAULT CURRENT_TIMESTAMP                 NOT NULL,
    deleted_at timestamp(0) DEFAULT NULL::timestamp without time zone NULL,
    tenant_id  varchar(16)                                            NOT NULL,
    CONSTRAINT sys_sys_role_id_scope_pkey PRIMARY KEY (id)
);
COMMENT ON TABLE public.sys_role_id_scope IS '角色数据 ID 范围表';

-- Column comments

COMMENT ON COLUMN public.sys_role_id_scope.id IS '主键 ID';
COMMENT ON COLUMN public.sys_role_id_scope.status IS '数据状态，0：禁用、1：启用';
COMMENT ON COLUMN public.sys_role_id_scope."type" IS '数据 ID 类型，1：用户ID、2：部门ID、3：用户组ID';
COMMENT ON COLUMN public.sys_role_id_scope.role_id IS '角色 ID';
COMMENT ON COLUMN public.sys_role_id_scope.data_id IS '数据 ID';
COMMENT ON COLUMN public.sys_role_id_scope.created_by IS '创建人';
COMMENT ON COLUMN public.sys_role_id_scope.created_at IS '创建时间';
COMMENT ON COLUMN public.sys_role_id_scope.updated_by IS '更新人';
COMMENT ON COLUMN public.sys_role_id_scope.updated_at IS '更新时间';
COMMENT ON COLUMN public.sys_role_id_scope.deleted_at IS '删除时间';
COMMENT ON COLUMN public.sys_role_id_scope.tenant_id IS '租户 ID';


-- public.sys_tenant 定义

-- Drop table

-- DROP TABLE public.sys_tenant;

CREATE TABLE public.sys_tenant
(
    id         varchar(16)                                            NOT NULL,
    status     int2         DEFAULT 1                                 NOT NULL,
    created_by int8                                                   NOT NULL,
    created_at timestamp(0) DEFAULT CURRENT_TIMESTAMP                 NOT NULL,
    updated_by int8                                                   NOT NULL,
    updated_at timestamp(0) DEFAULT CURRENT_TIMESTAMP                 NOT NULL,
    deleted_at timestamp(0) DEFAULT NULL::timestamp without time zone NULL,
    tenant_id  varchar(16)                                            NOT NULL,
    code       varchar(32)                                            NOT NULL,
    "name"     varchar(256)                                           NOT NULL,
    CONSTRAINT sys_tenant_name_un UNIQUE (name),
    CONSTRAINT sys_tenant_pk PRIMARY KEY (id),
    CONSTRAINT sys_tenant_un UNIQUE (code)
);
COMMENT ON TABLE public.sys_tenant IS '租户表';

-- Column comments

COMMENT ON COLUMN public.sys_tenant.id IS '租户 ID';
COMMENT ON COLUMN public.sys_tenant.status IS '数据状态，0：禁用、1：启用';
COMMENT ON COLUMN public.sys_tenant.created_by IS '创建人';
COMMENT ON COLUMN public.sys_tenant.created_at IS '创建时间';
COMMENT ON COLUMN public.sys_tenant.updated_by IS '更新人';
COMMENT ON COLUMN public.sys_tenant.updated_at IS '更新时间';
COMMENT ON COLUMN public.sys_tenant.deleted_at IS '删除时间';
COMMENT ON COLUMN public.sys_tenant.tenant_id IS '租户 ID';
COMMENT ON COLUMN public.sys_tenant.code IS '租户编码';
COMMENT ON COLUMN public.sys_tenant."name" IS '租户名';


-- public.sys_user 定义

-- Drop table

-- DROP TABLE public.sys_user;

CREATE TABLE public.sys_user
(
    id                  int8                                                   NOT NULL,
    status              int2         DEFAULT 1                                 NOT NULL,
    created_by          int8                                                   NOT NULL,
    created_at          timestamp(0) DEFAULT CURRENT_TIMESTAMP                 NOT NULL,
    updated_by          int8                                                   NOT NULL,
    updated_at          timestamp(0) DEFAULT CURRENT_TIMESTAMP                 NOT NULL,
    deleted_at          timestamp(0) DEFAULT NULL::timestamp without time zone NULL,
    tenant_id           varchar(16)                                            NULL,
    owner_user_id       int8                                                   NULL,
    owner_dept_id       int8                                                   NULL,
    owner_user_group_id int8                                                   NULL,
    "name"              varchar(256)                                           NOT NULL,
    nickname            varchar(256)                                           NOT NULL,
    "password"          text                                                   NOT NULL,
    remark              varchar                                                NULL,
    CONSTRAINT sys_user_pkey PRIMARY KEY (id)
);
CREATE UNIQUE INDEX sys_user_username_idx ON public.sys_user USING btree (name);
COMMENT ON TABLE public.sys_user IS '用户表';

-- Column comments

COMMENT ON COLUMN public.sys_user.id IS '用户 ID';
COMMENT ON COLUMN public.sys_user.status IS '数据状态，0：禁用、1：启用';
COMMENT ON COLUMN public.sys_user.created_by IS '创建人';
COMMENT ON COLUMN public.sys_user.created_at IS '创建时间';
COMMENT ON COLUMN public.sys_user.updated_by IS '更新人';
COMMENT ON COLUMN public.sys_user.updated_at IS '更新时间';
COMMENT ON COLUMN public.sys_user.deleted_at IS '删除时间';
COMMENT ON COLUMN public.sys_user.tenant_id IS '租户 ID';
COMMENT ON COLUMN public.sys_user.owner_user_id IS '归属用户 ID';
COMMENT ON COLUMN public.sys_user.owner_dept_id IS '归属部门 ID';
COMMENT ON COLUMN public.sys_user.owner_user_group_id IS '归属用户组 ID';
COMMENT ON COLUMN public.sys_user."name" IS '用户名';
COMMENT ON COLUMN public.sys_user.nickname IS '昵称';
COMMENT ON COLUMN public.sys_user."password" IS '密码';
COMMENT ON COLUMN public.sys_user.remark IS '备注';


-- public.user_role_mapping 定义

-- Drop table

-- DROP TABLE public.user_role_mapping;

CREATE TABLE public.user_role_mapping
(
    user_id int8 NOT NULL,
    role_id int8 NOT NULL,
    CONSTRAINT user_role_mapping_pk PRIMARY KEY (user_id, role_id)
);

-- Column comments

COMMENT ON COLUMN public.user_role_mapping.user_id IS '用户 ID';
COMMENT ON COLUMN public.user_role_mapping.role_id IS '角色 ID';


-- public.sys_user_group 定义

-- Drop table

-- DROP TABLE public.sys_user_group;

CREATE TABLE public.sys_user_group
(
    id            int8                                                   NOT NULL,
    status        int2         DEFAULT 1                                 NOT NULL,
    created_by    int8                                                   NOT NULL,
    created_at    timestamp(0) DEFAULT CURRENT_TIMESTAMP                 NOT NULL,
    updated_by    int8                                                   NOT NULL,
    updated_at    timestamp(0) DEFAULT CURRENT_TIMESTAMP                 NOT NULL,
    deleted_at    timestamp(0) DEFAULT NULL::timestamp without time zone NULL,
    tenant_id     varchar(16)                                            NOT NULL,
    "name"        varchar(256)                                           NOT NULL,
    code          varchar(64)                                            NOT NULL,
    owner_user_id int8                                                   NULL,
    remark        varchar(512)                                           NULL,
    CONSTRAINT sys_user_group_pkey PRIMARY KEY (id),
    CONSTRAINT sys_user_group_tenant_code_un UNIQUE (tenant_id, code),
    CONSTRAINT sys_user_group_tenant_name_un UNIQUE (tenant_id, name)
);
COMMENT ON TABLE public.sys_user_group IS '用户组表';

-- Column comments

COMMENT ON COLUMN public.sys_user_group.id IS '用户组 ID';
COMMENT ON COLUMN public.sys_user_group.status IS '数据状态，0：禁用、1：启用';
COMMENT ON COLUMN public.sys_user_group.created_by IS '创建人';
COMMENT ON COLUMN public.sys_user_group.created_at IS '创建时间';
COMMENT ON COLUMN public.sys_user_group.updated_by IS '更新人';
COMMENT ON COLUMN public.sys_user_group.updated_at IS '更新时间';
COMMENT ON COLUMN public.sys_user_group.deleted_at IS '删除时间';
COMMENT ON COLUMN public.sys_user_group.tenant_id IS '租户 ID';
COMMENT ON COLUMN public.sys_user_group."name" IS '组名';
COMMENT ON COLUMN public.sys_user_group.code IS '组编码';
COMMENT ON COLUMN public.sys_user_group.owner_user_id IS '组长/归属用户';
COMMENT ON COLUMN public.sys_user_group.remark IS '备注';


-- public.sys_user_group_mapping 定义

-- Drop table

-- DROP TABLE public.sys_user_group_mapping;

CREATE TABLE public.sys_user_group_mapping
(
    group_id int8 NOT NULL,
    user_id  int8 NOT NULL,
    CONSTRAINT sys_user_group_mapping_pk PRIMARY KEY (group_id, user_id)
);

-- Column comments

COMMENT ON COLUMN public.sys_user_group_mapping.group_id IS '用户组 ID';
COMMENT ON COLUMN public.sys_user_group_mapping.user_id IS '用户 ID';


-- basic 表
CREATE POLICY tenant_isolation_policy
    ON public.basic
    USING (
    tenant_id = current_setting('app.tenant_id', true)
    )
    WITH CHECK (
    tenant_id = current_setting('app.tenant_id', true)
    );

-- sys_dept
CREATE POLICY tenant_isolation_policy
    ON public.sys_dept
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));

-- sys_role
CREATE POLICY tenant_isolation_policy
    ON public.sys_role
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));

-- sys_role_id_scope
CREATE POLICY tenant_isolation_policy
    ON public.sys_role_id_scope
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));

-- sys_user
CREATE POLICY tenant_isolation_policy
    ON public.sys_user
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));

-- sys_user_group
CREATE POLICY tenant_isolation_policy
    ON public.sys_user_group
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));

ALTER TABLE public.basic ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.basic FORCE ROW LEVEL SECURITY;

ALTER TABLE public.sys_dept ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.sys_dept FORCE ROW LEVEL SECURITY;

ALTER TABLE public.sys_role ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.sys_role FORCE ROW LEVEL SECURITY;

ALTER TABLE public.sys_role_id_scope ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.sys_role_id_scope FORCE ROW LEVEL SECURITY;

ALTER TABLE public.sys_user ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.sys_user FORCE ROW LEVEL SECURITY;

ALTER TABLE public.sys_user_group ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.sys_user_group FORCE ROW LEVEL SECURITY;

-- 1. 创建用户
CREATE USER admin WITH PASSWORD 'admin';

-- 2. 授予登录权限
ALTER USER admin WITH LOGIN;

-- 3. 授予数据库级别权限
GRANT CONNECT ON DATABASE orm_test TO admin;

GRANT USAGE ON SCHEMA public TO admin;

GRANT SELECT, INSERT, UPDATE, DELETE
    ON ALL TABLES IN SCHEMA public
    TO admin;

GRANT USAGE, SELECT
    ON ALL SEQUENCES IN SCHEMA public
    TO admin;

ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE
    ON TABLES
    TO admin;

ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT USAGE, SELECT
    ON SEQUENCES
    TO admin;

GRANT SET ON PARAMETER app.tenant_id TO admin;