DROP INDEX IF EXISTS cloud_chunks_school_provider;
DROP INDEX IF EXISTS cloud_chunks_school_item_chunk;
DROP TABLE IF EXISTS cloud_chunks;

ALTER TABLE cloud_items
    DROP CONSTRAINT IF EXISTS cloud_items_chunk_count_nonnegative,
    DROP CONSTRAINT IF EXISTS cloud_items_index_status_valid,
    DROP COLUMN IF EXISTS indexed_at,
    DROP COLUMN IF EXISTS chunk_count,
    DROP COLUMN IF EXISTS index_provider,
    DROP COLUMN IF EXISTS index_error,
    DROP COLUMN IF EXISTS index_status;