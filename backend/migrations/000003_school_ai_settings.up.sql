-- 学校级共享 AI 模型配置：管理员统一维护厂商、接口地址、密钥与可用模型。
CREATE TABLE school_ai_settings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id uuid NOT NULL REFERENCES schools(id) ON DELETE CASCADE,
    provider varchar(30) NOT NULL DEFAULT 'custom',
    base_url varchar(500) NOT NULL DEFAULT '',
    api_key text NOT NULL DEFAULT '',
    models jsonb NOT NULL DEFAULT '[]'::jsonb,
    default_model varchar(120) NOT NULL DEFAULT '',
    enabled boolean NOT NULL DEFAULT false,
    updated_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    CONSTRAINT school_ai_settings_models_array CHECK (jsonb_typeof(models) = 'array'),
    CONSTRAINT school_ai_settings_provider_valid CHECK (provider IN ('openai', 'deepseek', 'qwen', 'zhipu', 'moonshot', 'custom')),
    CONSTRAINT school_ai_settings_base_url_valid CHECK (base_url = '' OR base_url ~ '^https?://'),
    CONSTRAINT school_ai_settings_enabled_ready CHECK (
        NOT enabled OR (btrim(base_url) <> '' AND btrim(api_key) <> '' AND btrim(default_model) <> '' AND jsonb_array_length(models) > 0)
    )
);

CREATE UNIQUE INDEX school_ai_settings_school_active ON school_ai_settings(school_id) WHERE deleted_at IS NULL;
