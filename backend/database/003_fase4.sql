-- MFA de TOTP. El secreto se cifra en la API antes de persistirlo.
ALTER TABLE users ADD COLUMN IF NOT EXISTS mfa_secret TEXT;
