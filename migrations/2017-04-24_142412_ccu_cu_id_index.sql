-- MDB generated migration file
-- rambler up

CREATE INDEX IF NOT EXISTS collections_content_units_content_unit_id_idx
  ON collections_content_units USING BTREE (content_unit_id);

-- rambler down

DROP INDEX IF EXISTS collections_content_units_content_unit_id_idx;