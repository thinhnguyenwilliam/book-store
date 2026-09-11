BEGIN;
CREATE TABLE IF NOT EXISTS auth.permissions (
 code text PRIMARY KEY, name text NOT NULL, description text NOT NULL DEFAULT '', group_name text NOT NULL
);
CREATE TABLE IF NOT EXISTS auth.roles (
 code text PRIMARY KEY, name text NOT NULL, description text NOT NULL DEFAULT '', system boolean NOT NULL DEFAULT false
);
CREATE TABLE IF NOT EXISTS auth.role_permissions (
 role_code text NOT NULL REFERENCES auth.roles(code) ON DELETE CASCADE,
 permission_code text NOT NULL REFERENCES auth.permissions(code), PRIMARY KEY(role_code, permission_code)
);
CREATE TABLE IF NOT EXISTS auth.account_roles (
 account_id uuid NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
 role_code text NOT NULL REFERENCES auth.roles(code), PRIMARY KEY(account_id, role_code)
);
CREATE INDEX IF NOT EXISTS account_roles_role_idx ON auth.account_roles(role_code, account_id);
CREATE TABLE IF NOT EXISTS auth.authorization_audit_logs (
 id bigserial PRIMARY KEY, actor_id text NOT NULL, action text NOT NULL, target text NOT NULL,
 before_value jsonb NOT NULL, after_value jsonb NOT NULL, trace_id text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO auth.permissions(code,name,group_name) VALUES
 ('admin.access','Truy cập back-office','admin'),
 ('books.read','Xem danh mục quản trị','books'),('books.create','Tạo sách','books'),
 ('books.update','Sửa sách và tồn kho','books'),('books.delete','Xóa sách','books'),
 ('customers.read','Xem khách hàng','customers'),('customers.update','Sửa khách hàng','customers'),('customers.delete','Xóa khách hàng','customers'),
 ('wallets.adjust','Điều chỉnh số dư ví','wallets'),('comments.moderate','Kiểm duyệt bình luận','comments'),
 ('analytics.read','Xem báo cáo','analytics'),('chat.read','Xem chat hỗ trợ','chat'),('chat.reply','Trả lời chat hỗ trợ','chat'),
 ('roles.read','Xem vai trò và quyền','authorization'),('roles.manage','Quản lý vai trò','authorization'),
 ('users.assign_roles','Gán vai trò tài khoản','authorization')
 ON CONFLICT DO NOTHING;
INSERT INTO auth.roles(code,name,system) VALUES ('customer','Khách hàng',true),('admin','Quản trị nghiệp vụ',true),
 ('super_admin','Quản trị quyền hệ thống',true),('support','Hỗ trợ khách hàng',true),('catalog_manager','Quản lý sách',true)
 ON CONFLICT DO NOTHING;
INSERT INTO auth.role_permissions SELECT 'super_admin',code FROM auth.permissions ON CONFLICT DO NOTHING;
INSERT INTO auth.role_permissions SELECT 'admin',code FROM auth.permissions WHERE group_name <> 'authorization' ON CONFLICT DO NOTHING;
INSERT INTO auth.role_permissions SELECT 'support',code FROM auth.permissions WHERE code IN ('admin.access','customers.read','chat.read','chat.reply') ON CONFLICT DO NOTHING;
INSERT INTO auth.role_permissions SELECT 'catalog_manager',code FROM auth.permissions WHERE group_name='books' OR code='admin.access' ON CONFLICT DO NOTHING;
-- One-time backfill only. Re-running migrations must never restore revoked roles.
DO $$ BEGIN
 IF NOT EXISTS (SELECT 1 FROM auth.authorization_audit_logs WHERE action='rbac.bootstrap') THEN
  INSERT INTO auth.account_roles SELECT a.id,r.code FROM auth.accounts a JOIN auth.roles r ON r.code=ANY(a.roles) ON CONFLICT DO NOTHING;
  -- Existing admins keep all prior business access. Oldest admin bootstraps role administration.
  INSERT INTO auth.account_roles SELECT id,'super_admin' FROM auth.accounts WHERE 'admin'=ANY(roles) ORDER BY created_at,id LIMIT 1 ON CONFLICT DO NOTHING;
  INSERT INTO auth.authorization_audit_logs(actor_id,action,target,before_value,after_value)
  VALUES ('migration','rbac.bootstrap','020_permissions','{}',COALESCE((SELECT jsonb_agg(account_id) FROM auth.account_roles WHERE role_code='super_admin'),'[]'));
 END IF;
END $$;
COMMIT;
