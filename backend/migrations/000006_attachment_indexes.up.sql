-- 删除会话时按会话回收附件；该索引避免全表扫描附件表。
CREATE INDEX ai_attachments_school_conversation ON ai_attachments(school_id, conversation_id)
    WHERE deleted_at IS NULL AND conversation_id IS NOT NULL;

-- 试卷列表按试卷聚合题目数量，补充覆盖索引减少回表。
CREATE INDEX paper_questions_school_paper_active ON paper_questions(school_id, paper_id)
    WHERE deleted_at IS NULL;
