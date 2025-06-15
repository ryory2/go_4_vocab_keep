ALTER TABLE public.tenants ADD COLUMN IF NOT EXISTS password_hash TEXT;

UPDATE public.tenants t
SET password_hash = i.password_hash
FROM public.identities i
WHERE t.tenant_id = i.tenant_id AND i.auth_provider = 'local';

ALTER TABLE public.tenants ADD CONSTRAINT tenants_name_key UNIQUE (name);

DROP TABLE IF EXISTS public.identities;