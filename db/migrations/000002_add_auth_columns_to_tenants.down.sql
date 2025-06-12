-- up のステップ4の逆: UNIQUE制約を削除する
-- 制約名は up で指定した 'tenants_email_unique' を使う
ALTER TABLE tenants
DROP CONSTRAINT IF EXISTS tenants_email_unique;

-- up のステップ3の逆: NOT NULL制約を削除する
-- （カラム自体を削除するので、厳密には不要ですが、明示的に書くこともできます）
-- ALTER TABLE vocab_keep.tenants
-- ALTER COLUMN email DROP NOT NULL,
-- ALTER COLUMN password_hash DROP NOT NULL;

-- up のステップ1の逆: カラムを削除する
ALTER TABLE tenants
DROP COLUMN IF EXISTS email,
DROP COLUMN IF EXISTS password_hash;