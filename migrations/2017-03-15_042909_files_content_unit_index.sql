-- MDB generated migration file

-- rambler up
CREATE INDEX IF NOT EXISTS files_content_unit_id_idx
  ON files USING BTREE (content_unit_id);

-- rambler down
DROP INDEX IF EXISTS files_content_unit_id_idx;