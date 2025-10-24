-- ===== Extensions (safe to re-run) =====
CREATE EXTENSION IF NOT EXISTS pgcrypto;   -- gen_random_uuid()
CREATE EXTENSION IF NOT EXISTS citext;     -- case-insensitive emails
CREATE EXTENSION IF NOT EXISTS pg_trgm;    -- fuzzy search on names/slugs (optional)

-- ===== Enums =====
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'project_visibility') THEN
    CREATE TYPE project_visibility AS ENUM ('private','unlisted','public');
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'chat_sender') THEN
    CREATE TYPE chat_sender AS ENUM ('user','assistant','system','tool');
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'auth_provider') THEN
    CREATE TYPE auth_provider AS ENUM ('password','google');
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'token_purpose') THEN
    CREATE TYPE token_purpose AS ENUM (
      'email_verify',      -- verify email ownership
      'email_login',       -- magic link login
      'phone_verify',      -- verify phone via OTP
      'phone_login',       -- OTP login
      'reset_password'     -- password reset flow
    );
  END IF;
END$$;

-- ===== Touch updated_at trigger =====
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END$$;

-- ===================== Core Tables =====================

-- users: account / profile (updated to support phone-only accounts)
CREATE TABLE IF NOT EXISTS users (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email          citext,                            -- NULL for phone-only accounts
  display_name   text,
  password_hash  text,                            -- or NULL if using SSO/OIDC
  is_active      boolean NOT NULL DEFAULT true,
  metadata       jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz
);
CREATE TRIGGER trg_users_updated_at
BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX IF NOT EXISTS idx_users_active
  ON users(is_active) WHERE deleted_at IS NULL;

-- Keep uniqueness on non-null emails
DO $$
BEGIN
  IF NOT EXISTS (
     SELECT 1 FROM pg_indexes WHERE indexname = 'uq_users_email_not_null'
  ) THEN
    CREATE UNIQUE INDEX uq_users_email_not_null ON users(email)
    WHERE email IS NOT NULL AND deleted_at IS NULL;
  END IF;
END$$;

-- ===================== Authentication Tables =====================

-- Phone numbers linked to users
CREATE TABLE IF NOT EXISTS user_phones (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  e164         text NOT NULL,                           -- +8869xxxxxxx (E.164)
  is_primary   boolean NOT NULL DEFAULT false,
  verified_at  timestamptz,
  created_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (e164)                                         -- one account per phone
);
CREATE INDEX IF NOT EXISTS idx_user_phones_user ON user_phones(user_id);

-- External identities (Google OAuth / OIDC, etc.)
CREATE TABLE IF NOT EXISTS auth_identities (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id         uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  provider        auth_provider NOT NULL,
  provider_uid    text NOT NULL,        -- OIDC sub / Google user ID
  email_at_signup citext,               -- email seen at link time
  email_verified  boolean,
  -- Optionally store opaque refresh token or just last 4 chars; keep minimal.
  refresh_token_encrypted bytea,        -- store only if you must
  created_at      timestamptz NOT NULL DEFAULT now(),
  UNIQUE (provider, provider_uid)
);
CREATE INDEX IF NOT EXISTS idx_auth_identities_user ON auth_identities(user_id);

-- Credentials for password / email login (optional)
CREATE TABLE IF NOT EXISTS user_credentials (
  user_id       uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  password_hash text,                        -- bcrypt/argon2id; NULL if social-only
  password_set_at timestamptz
);

-- One-time / magic tokens (email & phone OTP/magic links)
CREATE TABLE IF NOT EXISTS verification_tokens (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id       uuid REFERENCES users(id) ON DELETE CASCADE,
  purpose       token_purpose NOT NULL,
  sent_to       text NOT NULL,               -- email or E.164 phone
  token_hash    text NOT NULL,               -- hash of OTP or magic token
  expires_at    timestamptz NOT NULL,
  consumed_at   timestamptz,
  attempt_count integer NOT NULL DEFAULT 0,
  metadata      jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_verification_tokens_lookup ON verification_tokens(purpose, sent_to, expires_at);

-- Sessions (web / api)
CREATE TABLE IF NOT EXISTS sessions (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id         uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  user_agent      text,
  ip_net          inet,
  refresh_token_hash text NOT NULL,          -- hash only, never raw
  valid_until     timestamptz NOT NULL,      -- rotation/expiry
  created_at      timestamptz NOT NULL DEFAULT now(),
  revoked_at      timestamptz
);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);

-- Audit (minimal but helpful)
CREATE TABLE IF NOT EXISTS auth_audit (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     uuid REFERENCES users(id) ON DELETE SET NULL,
  event       text NOT NULL,                -- 'login_success','otp_sent','logout', etc.
  provider    auth_provider,
  ip_net      inet,
  user_agent  text,
  details     jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at  timestamptz NOT NULL DEFAULT now()
);

-- ===================== Project Tables =====================

-- projects: each project has a MinIO root (bucket + prefix)
CREATE TABLE IF NOT EXISTS projects (
  id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_id             uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name                 text NOT NULL,
  slug                 text GENERATED ALWAYS AS
                       (regexp_replace(lower(name), '[^a-z0-9]+', '-', 'g')) STORED,
  description          text,
  visibility           project_visibility NOT NULL DEFAULT 'private',

  -- MinIO mapping
  root_bucket          text NOT NULL,         -- e.g. 'webapps'
  root_prefix          text NOT NULL,         -- e.g. 'users/{user_id}/projects/{project_id}/'

  storage_quota_bytes  bigint NOT NULL DEFAULT 0,  -- 0 = unlimited (enforced in app)
  meta                 jsonb  NOT NULL DEFAULT '{}'::jsonb,
  version              integer NOT NULL DEFAULT 1,  -- optimistic lock (optional)

  created_at           timestamptz NOT NULL DEFAULT now(),
  updated_at           timestamptz NOT NULL DEFAULT now(),
  deleted_at           timestamptz,

  CONSTRAINT uq_projects_owner_name UNIQUE (owner_id, name)
);
CREATE TRIGGER trg_projects_updated_at
BEFORE UPDATE ON projects FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX IF NOT EXISTS idx_projects_owner
  ON projects(owner_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_projects_slug_trgm
  ON projects USING gin (slug gin_trgm_ops);

-- ===================== Session Management (Chat) =====================

-- A chat session is usually tied to a project and a user
CREATE TABLE IF NOT EXISTS chat_sessions (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  project_id   uuid REFERENCES projects(id) ON DELETE CASCADE,
  title        text,
  model_hint   text,               -- optional: model name you prefer for this session
  meta         jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  archived_at  timestamptz
);
CREATE TRIGGER trg_chat_sessions_updated_at
BEFORE UPDATE ON chat_sessions FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX IF NOT EXISTS idx_chat_sessions_user_time
  ON chat_sessions(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_chat_sessions_project_time
  ON chat_sessions(project_id, created_at DESC);

-- Messages inside a session (ordered by created_at)
CREATE TABLE IF NOT EXISTS chat_messages (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  session_id    uuid NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
  sender        chat_sender NOT NULL,
  text          text,              -- human-readable message text
  content       jsonb NOT NULL,    -- rich content: blocks, code, tool I/O, etc.
  tokens_used   integer,           -- optional accounting
  tool_name     text,              -- if sender='tool'
  reply_to      uuid REFERENCES chat_messages(id) ON DELETE SET NULL,
  created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_chat_messages_session_time
  ON chat_messages(session_id, created_at);
CREATE INDEX IF NOT EXISTS idx_chat_messages_reply_to
  ON chat_messages(reply_to);

-- ===================== Convenience Views =====================
CREATE OR REPLACE VIEW active_users AS
  SELECT * FROM users WHERE is_active = true AND deleted_at IS NULL;

CREATE OR REPLACE VIEW active_projects AS
  SELECT * FROM projects WHERE deleted_at IS NULL;
