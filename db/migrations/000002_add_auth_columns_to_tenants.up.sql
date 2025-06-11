-- ステップ1: カラムを追加し、一時的なデフォルト値を設定する
-- emailはUNIQUE制約があるため、全ての行で同じ値は使えない。
-- ここでは一意性を保つために、既存のtenant_idを流用するなどの工夫が必要。
-- 最も簡単なのは、一時的にNOT NULLを外すことです。
ALTER TABLE tenants
ADD COLUMN email TEXT,
ADD COLUMN password_hash TEXT,
ADD COLUMN is_active BOOLEAN NOT NULL DEFAULT false;

-- ステップ2: 既存のデータに値をUPDATEする
-- ★★★注意: ここで設定する値は、あなたのアプリケーションの仕様に合わせてください。★★★
-- 例えば、既存テナントはダミーのメールアドレスとパスワードを持つことにします。
UPDATE tenants
SET 
  email = 'temp-' || tenant_id::text || '@example.com', -- tenant_idを使って一意なメールアドレスを生成
  password_hash = 'temporary-password-hash',             -- 仮のハッシュ値
  is_active = true 
WHERE 
  email IS NULL; -- まだ設定されていない行のみを対象

-- ステップ3: カラムに NOT NULL 制約を追加する
ALTER TABLE tenants
ALTER COLUMN email SET NOT NULL,
ALTER COLUMN password_hash SET NOT NULL;

-- ステップ4: emailカラムにUNIQUE制約を追加する
ALTER TABLE tenants
ADD CONSTRAINT tenants_email_unique UNIQUE (email);