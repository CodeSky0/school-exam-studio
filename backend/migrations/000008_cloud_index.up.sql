-- 云盘资料自动向量化：文件索引状态与文本分块向量。
ALTER TABLE cloud_items
    ADD COLUMN index_status varchar(20) NOT NULL DEFAULT 'pending',
    ADD COLUMN index_error varchar(500) NOT NULL DEFAULT '',
    ADD COLUMN index_provider varchar(80) NOT NULL DEFAULT '',
    ADD COLUMN chunk_count integer NOT NULL DEFAULT 0,
    ADD COLUMN indexed_at timestamptz;

ALTER TABLE cloud_items
    ADD CONSTRAINT cloud_items_index_status_valid
        CHECK (index_status IN ('pending', 'processing', 'ready', 'unsupported', 'failed')),
    ADD CONSTRAINT cloud_items_chunk_count_nonnegative CHECK (chunk_count >= 0);

CREATE TABLE cloud_chunks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id uuid NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    cloud_item_id uuid NOT NULL,
    chunk_index integer NOT NULL DEFAULT 0,
    content varchar(8000) NOT NULL,
    embedding jsonb NOT NULL,
    provider varchar(80) NOT NULL DEFAULT '',
    char_count integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    CONSTRAINT cloud_chunks_content_not_blank CHECK (btrim(content) <> ''),
    CONSTRAINT cloud_chunks_chunk_index_nonnegative CHECK (chunk_index >= 0),
    CONSTRAINT cloud_chunks_char_count_nonnegative CHECK (char_count >= 0),
    CONSTRAINT cloud_chunks_embedding_array CHECK (jsonb_typeof(embedding) = 'array'),
    CONSTRAINT cloud_chunks_school_id_id_key UNIQUE (school_id, id),
    CONSTRAINT cloud_chunks_item_fk FOREIGN KEY (school_id, cloud_item_id)
        REFERENCES cloud_items(school_id, id) ON DELETE CASCADE
);

CREATE INDEX cloud_chunks_school_item_chunk ON cloud_chunks(school_id, cloud_item_id, chunk_index)
    WHERE deleted_at IS NULL;
CREATE INDEX cloud_chunks_school_provider ON cloud_chunks(school_id, provider)
    WHERE deleted_at IS NULL;