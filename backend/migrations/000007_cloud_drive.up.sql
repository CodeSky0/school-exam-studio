CREATE TABLE cloud_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id uuid NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    creator_id uuid NOT NULL,
    parent_id uuid,
    name varchar(255) NOT NULL,
    is_folder boolean NOT NULL DEFAULT false,
    mime varchar(120) NOT NULL DEFAULT '',
    kind varchar(20) NOT NULL DEFAULT '',
    size_bytes bigint NOT NULL DEFAULT 0,
    storage_path varchar(500) NOT NULL DEFAULT '',
    sha256 varchar(64) NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    CONSTRAINT cloud_items_name_not_blank CHECK (btrim(name) <> ''),
    CONSTRAINT cloud_items_size_nonnegative CHECK (size_bytes >= 0),
    CONSTRAINT cloud_items_folder_no_storage CHECK (NOT is_folder OR (storage_path = '' AND size_bytes = 0)),
    CONSTRAINT cloud_items_school_id_id_key UNIQUE (school_id, id),
    CONSTRAINT cloud_items_parent_fk FOREIGN KEY (school_id, parent_id) REFERENCES cloud_items(school_id, id) ON DELETE CASCADE,
    CONSTRAINT cloud_items_creator_school_fk FOREIGN KEY (school_id, creator_id) REFERENCES users(school_id, id) ON DELETE RESTRICT
);

CREATE INDEX cloud_items_school_parent_created ON cloud_items(school_id, parent_id, is_folder DESC, name) WHERE deleted_at IS NULL;
CREATE INDEX cloud_items_school_sha256 ON cloud_items(school_id, sha256) WHERE deleted_at IS NULL;
