CREATE TABLE ai_conversations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id uuid NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    creator_id uuid NOT NULL,
    title varchar(120) NOT NULL DEFAULT '新对话',
    status varchar(20) NOT NULL DEFAULT 'active',
    pinned boolean NOT NULL DEFAULT false,
    message_count int NOT NULL DEFAULT 0,
    last_message_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    CONSTRAINT ai_conversations_status_valid CHECK (status IN ('active', 'archived')),
    CONSTRAINT ai_conversations_title_not_blank CHECK (btrim(title) <> ''),
    CONSTRAINT ai_conversations_school_id_id_key UNIQUE (school_id, id),
    CONSTRAINT ai_conversations_creator_school_fk FOREIGN KEY (school_id, creator_id) REFERENCES users(school_id, id) ON DELETE RESTRICT
);

CREATE TABLE ai_messages (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id uuid NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    conversation_id uuid NOT NULL,
    role varchar(20) NOT NULL,
    content text NOT NULL DEFAULT '',
    status varchar(20) NOT NULL DEFAULT 'succeeded',
    provider varchar(20) NOT NULL DEFAULT 'local',
    model varchar(80) NOT NULL DEFAULT '',
    tool_calls jsonb NOT NULL DEFAULT '[]'::jsonb,
    attachments jsonb NOT NULL DEFAULT '[]'::jsonb,
    run_id uuid,
    error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    CONSTRAINT ai_messages_role_valid CHECK (role IN ('user', 'assistant', 'tool', 'system')),
    CONSTRAINT ai_messages_status_valid CHECK (status IN ('running', 'succeeded', 'failed')),
    CONSTRAINT ai_messages_tool_calls_array CHECK (jsonb_typeof(tool_calls) = 'array'),
    CONSTRAINT ai_messages_attachments_array CHECK (jsonb_typeof(attachments) = 'array'),
    CONSTRAINT ai_messages_conversation_fk FOREIGN KEY (school_id, conversation_id) REFERENCES ai_conversations(school_id, id) ON DELETE CASCADE
);

CREATE TABLE ai_attachments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id uuid NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    creator_id uuid NOT NULL,
    conversation_id uuid,
    name varchar(255) NOT NULL,
    mime varchar(120) NOT NULL,
    kind varchar(20) NOT NULL,
    size_bytes bigint NOT NULL,
    storage_path varchar(500) NOT NULL DEFAULT '',
    sha256 varchar(64) NOT NULL DEFAULT '',
    extracted_text text NOT NULL DEFAULT '',
    text_chars int NOT NULL DEFAULT 0,
    status varchar(20) NOT NULL DEFAULT 'uploaded',
    error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    CONSTRAINT ai_attachments_kind_valid CHECK (kind IN ('image', 'pdf', 'docx', 'text')),
    CONSTRAINT ai_attachments_status_valid CHECK (status IN ('uploaded', 'parsed', 'failed')),
    CONSTRAINT ai_attachments_size_positive CHECK (size_bytes > 0),
    CONSTRAINT ai_attachments_creator_school_fk FOREIGN KEY (school_id, creator_id) REFERENCES users(school_id, id) ON DELETE RESTRICT
);

CREATE INDEX ai_conversations_school_creator_pinned ON ai_conversations(school_id, creator_id, pinned DESC, last_message_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX ai_conversations_school_last_message ON ai_conversations(school_id, last_message_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX ai_messages_school_conversation_created ON ai_messages(school_id, conversation_id, created_at) WHERE deleted_at IS NULL;
CREATE INDEX ai_attachments_school_creator_created ON ai_attachments(school_id, creator_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX ai_attachments_school_sha256 ON ai_attachments(school_id, sha256) WHERE deleted_at IS NULL;