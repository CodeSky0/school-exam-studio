DROP TABLE IF EXISTS ai_audit_logs;
DROP TABLE IF EXISTS ai_runs;

DROP INDEX IF EXISTS questions_school_source_question;
DROP INDEX IF EXISTS questions_school_content_hash;
DROP INDEX IF EXISTS questions_school_category;
DROP INDEX IF EXISTS questions_knowledge_points_gin;

ALTER TABLE questions
    DROP CONSTRAINT IF EXISTS questions_ai_meta_object,
    DROP CONSTRAINT IF EXISTS questions_knowledge_points_array;

ALTER TABLE questions
    DROP COLUMN IF EXISTS source_question_id,
    DROP COLUMN IF EXISTS content_hash,
    DROP COLUMN IF EXISTS ai_meta,
    DROP COLUMN IF EXISTS ai_generated,
    DROP COLUMN IF EXISTS knowledge_points,
    DROP COLUMN IF EXISTS category;
