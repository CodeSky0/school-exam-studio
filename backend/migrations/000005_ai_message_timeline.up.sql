ALTER TABLE ai_messages ADD COLUMN timeline jsonb NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE ai_messages ADD CONSTRAINT ai_messages_timeline_array CHECK (jsonb_typeof(timeline) = 'array');