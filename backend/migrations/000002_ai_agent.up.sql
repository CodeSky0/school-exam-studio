ALTER TABLE questions
    ADD COLUMN category varchar(60) NOT NULL DEFAULT '',
    ADD COLUMN knowledge_points jsonb NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN ai_generated boolean NOT NULL DEFAULT false,
    ADD COLUMN ai_meta jsonb NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN content_hash varchar(64) NOT NULL DEFAULT '',
    ADD COLUMN source_question_id uuid;

ALTER TABLE questions
    ADD CONSTRAINT questions_knowledge_points_array CHECK (jsonb_typeof(knowledge_points) = 'array'),
    ADD CONSTRAINT questions_ai_meta_object CHECK (jsonb_typeof(ai_meta) = 'object');

CREATE INDEX questions_knowledge_points_gin ON questions USING gin(knowledge_points);
CREATE INDEX questions_school_category ON questions(school_id, category) WHERE deleted_at IS NULL;
CREATE INDEX questions_school_content_hash ON questions(school_id, content_hash) WHERE deleted_at IS NULL;
CREATE INDEX questions_school_source_question ON questions(school_id, source_question_id) WHERE deleted_at IS NULL;

CREATE TABLE ai_runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id uuid NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    creator_id uuid NOT NULL,
    goal text NOT NULL,
    intent varchar(40) NOT NULL DEFAULT 'unknown',
    status varchar(20) NOT NULL DEFAULT 'succeeded',
    provider varchar(20) NOT NULL DEFAULT 'local',
    model varchar(80) NOT NULL DEFAULT '',
    plan jsonb NOT NULL DEFAULT '[]'::jsonb,
    steps jsonb NOT NULL DEFAULT '[]'::jsonb,
    result jsonb NOT NULL DEFAULT '{}'::jsonb,
    error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    CONSTRAINT ai_runs_status_valid CHECK (status IN ('succeeded', 'failed')),
    CONSTRAINT ai_runs_goal_not_blank CHECK (btrim(goal) <> ''),
    CONSTRAINT ai_runs_plan_array CHECK (jsonb_typeof(plan) = 'array'),
    CONSTRAINT ai_runs_steps_array CHECK (jsonb_typeof(steps) = 'array'),
    CONSTRAINT ai_runs_result_object CHECK (jsonb_typeof(result) = 'object'),
    CONSTRAINT ai_runs_school_id_id_key UNIQUE (school_id, id),
    CONSTRAINT ai_runs_creator_school_fk FOREIGN KEY (school_id, creator_id) REFERENCES users(school_id, id) ON DELETE RESTRICT
);

CREATE TABLE ai_audit_logs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id uuid NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    actor_id uuid NOT NULL,
    run_id uuid REFERENCES ai_runs(id) ON DELETE SET NULL,
    action varchar(60) NOT NULL,
    target_type varchar(30) NOT NULL,
    target_id uuid,
    detail jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    CONSTRAINT ai_audit_logs_action_not_blank CHECK (btrim(action) <> ''),
    CONSTRAINT ai_audit_logs_target_type_not_blank CHECK (btrim(target_type) <> ''),
    CONSTRAINT ai_audit_logs_detail_object CHECK (jsonb_typeof(detail) = 'object'),
    CONSTRAINT ai_audit_logs_actor_school_fk FOREIGN KEY (school_id, actor_id) REFERENCES users(school_id, id) ON DELETE RESTRICT
);

CREATE INDEX ai_runs_school_creator_created ON ai_runs(school_id, creator_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX ai_runs_school_status ON ai_runs(school_id, status) WHERE deleted_at IS NULL;
CREATE INDEX ai_audit_logs_school_created ON ai_audit_logs(school_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX ai_audit_logs_school_actor_created ON ai_audit_logs(school_id, actor_id, created_at DESC) WHERE deleted_at IS NULL;
