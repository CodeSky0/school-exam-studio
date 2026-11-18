ALTER TABLE ai_messages DROP CONSTRAINT IF EXISTS ai_messages_timeline_array;

ALTER TABLE ai_messages DROP COLUMN IF EXISTS timeline;