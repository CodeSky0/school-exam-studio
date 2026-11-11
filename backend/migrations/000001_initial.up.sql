CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE schools (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name varchar(200) NOT NULL,
    school_code varchar(8) NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    CONSTRAINT schools_name_not_blank CHECK (btrim(name) <> ''),
    CONSTRAINT schools_school_code_format CHECK (school_code ~ '^[0-9]{8}$')
);

CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id uuid NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    role varchar(20) NOT NULL,
    name varchar(50) NOT NULL,
    subject varchar(30) NOT NULL DEFAULT '',
    email varchar(320) NOT NULL,
    password_hash text NOT NULL,
    status varchar(20) NOT NULL DEFAULT 'active',
    session_version integer NOT NULL DEFAULT 1 CHECK (session_version >= 1),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    CONSTRAINT users_role_valid CHECK (role IN ('admin', 'teacher')),
    CONSTRAINT users_status_valid CHECK (status IN ('active', 'disabled')),
    CONSTRAINT users_name_not_blank CHECK (btrim(name) <> ''),
    CONSTRAINT users_email_not_blank CHECK (btrim(email) <> ''),
    CONSTRAINT users_school_email_key UNIQUE (school_id, email),
    CONSTRAINT users_school_id_id_key UNIQUE (school_id, id)
);

CREATE TABLE questions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id uuid NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    creator_id uuid NOT NULL,
    subject varchar(30) NOT NULL,
    grade varchar(30) NOT NULL,
    question_type varchar(30) NOT NULL,
    difficulty integer NOT NULL CHECK (difficulty BETWEEN 1 AND 5),
    tags jsonb NOT NULL DEFAULT '[]'::jsonb,
    content text NOT NULL,
    options jsonb NOT NULL DEFAULT '[]'::jsonb,
    answer text NOT NULL DEFAULT '',
    explanation text NOT NULL DEFAULT '',
    status varchar(20) NOT NULL DEFAULT 'draft',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    CONSTRAINT questions_type_valid CHECK (question_type IN ('single_choice', 'multiple_choice', 'true_false', 'fill_blank', 'essay')),
    CONSTRAINT questions_status_valid CHECK (status IN ('draft', 'published', 'archived')),
    CONSTRAINT questions_subject_not_blank CHECK (btrim(subject) <> ''),
    CONSTRAINT questions_grade_not_blank CHECK (btrim(grade) <> ''),
    CONSTRAINT questions_content_not_blank CHECK (btrim(content) <> ''),
    CONSTRAINT questions_tags_array CHECK (jsonb_typeof(tags) = 'array'),
    CONSTRAINT questions_options_array CHECK (jsonb_typeof(options) = 'array'),
    CONSTRAINT questions_school_id_id_key UNIQUE (school_id, id),
    CONSTRAINT questions_creator_school_fk FOREIGN KEY (school_id, creator_id) REFERENCES users(school_id, id) ON DELETE RESTRICT
);

CREATE TABLE papers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id uuid NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    creator_id uuid NOT NULL,
    title varchar(200) NOT NULL,
    subject varchar(30) NOT NULL,
    grade varchar(30) NOT NULL,
    total_score numeric(8,2) NOT NULL DEFAULT 0.00 CHECK (total_score >= 0),
    duration_minutes integer NOT NULL CHECK (duration_minutes BETWEEN 1 AND 600),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    CONSTRAINT papers_title_not_blank CHECK (btrim(title) <> ''),
    CONSTRAINT papers_subject_not_blank CHECK (btrim(subject) <> ''),
    CONSTRAINT papers_grade_not_blank CHECK (btrim(grade) <> ''),
    CONSTRAINT papers_school_id_id_key UNIQUE (school_id, id),
    CONSTRAINT papers_creator_school_fk FOREIGN KEY (school_id, creator_id) REFERENCES users(school_id, id) ON DELETE RESTRICT
);

CREATE TABLE paper_questions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id uuid NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    paper_id uuid NOT NULL,
    question_id uuid NOT NULL,
    order_index integer NOT NULL CHECK (order_index >= 0),
    assigned_score numeric(8,2) NOT NULL CHECK (assigned_score > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    CONSTRAINT paper_questions_paper_school_fk FOREIGN KEY (school_id, paper_id) REFERENCES papers(school_id, id) ON DELETE RESTRICT,
    CONSTRAINT paper_questions_question_school_fk FOREIGN KEY (school_id, question_id) REFERENCES questions(school_id, id) ON DELETE RESTRICT
);

CREATE UNIQUE INDEX paper_questions_paper_order_active ON paper_questions(paper_id, order_index) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX paper_questions_paper_question_active ON paper_questions(paper_id, question_id) WHERE deleted_at IS NULL;
CREATE INDEX questions_school_filters ON questions(school_id, subject, grade, question_type) WHERE deleted_at IS NULL;
CREATE INDEX questions_school_creator_status ON questions(school_id, creator_id, status) WHERE deleted_at IS NULL;
CREATE INDEX questions_tags_gin ON questions USING gin(tags);
CREATE INDEX papers_school_creator_updated ON papers(school_id, creator_id, updated_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX paper_questions_school_paper_order ON paper_questions(school_id, paper_id, order_index) WHERE deleted_at IS NULL;
CREATE INDEX users_school_role_subject ON users(school_id, role, subject) WHERE deleted_at IS NULL;
