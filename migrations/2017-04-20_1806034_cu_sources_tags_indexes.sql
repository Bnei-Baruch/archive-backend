-- MDB generated migration file
-- rambler up

CREATE INDEX IF NOT EXISTS content_units_sources_source_id_idx
  ON content_units_sources USING BTREE (source_id);


CREATE INDEX IF NOT EXISTS content_units_tags_tag_id_idx
  ON content_units_tags USING BTREE (tag_id);

-- rambler down

DROP INDEX IF EXISTS content_units_sources_source_id_idx;
DROP INDEX IF EXISTS content_units_tags_tag_id_idx;