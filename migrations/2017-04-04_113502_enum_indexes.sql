-- MDB generated migration file
-- rambler up

CREATE INDEX IF NOT EXISTS collections_content_type_idx
  ON collections USING BTREE (type_id);

CREATE INDEX IF NOT EXISTS content_units_content_type_idx
  ON content_units USING BTREE (type_id);

CREATE INDEX IF NOT EXISTS operations_content_type_idx
  ON operations USING BTREE (type_id);

-- rambler down

DROP INDEX IF EXISTS collections_content_type_idx;
DROP INDEX IF EXISTS content_units_content_type_idx;
DROP INDEX IF EXISTS operations_content_type_idx;