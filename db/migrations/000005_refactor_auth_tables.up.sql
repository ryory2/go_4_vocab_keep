CREATE TABLE IF NOT EXISTS public.identities (
    id SERIAL PRIMARY KEY,
    tenant_id uuid NOT NULL,
    auth_provider VARCHAR(50) NOT NULL,
    provider_id TEXT NOT NULL,
    password_hash TEXT,
    
    -- 外部キー制約を追加
    CONSTRAINT fk_tenants
        FOREIGN KEY(tenant_id) 
        REFERENCES tenants(tenant_id)
        ON DELETE CASCADE,

    CONSTRAINT uq_identity_provider UNIQUE (auth_provider, provider_id)
);

CREATE INDEX IF NOT EXISTS idx_identities_tenant_id ON public.identities (tenant_id);

INSERT INTO public.identities (tenant_id, auth_provider, provider_id, password_hash)
SELECT 
    tenant_id,
    'local' AS auth_provider,
    email AS provider_id,
    password_hash
FROM 
    public.tenants
ON CONFLICT (auth_provider, provider_id) DO NOTHING;

ALTER TABLE public.tenants DROP COLUMN IF EXISTS password_hash;

ALTER TABLE public.tenants DROP CONSTRAINT IF EXISTS tenants_name_key;