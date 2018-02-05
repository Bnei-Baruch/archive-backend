-- MDB generated migration file
-- rambler up

DROP TABLE IF EXISTS content_units_tags;
CREATE TABLE content_units_tags (
  content_unit_id BIGINT REFERENCES content_units NOT NULL,
  tag_id          BIGINT REFERENCES tags          NOT NULL,
  PRIMARY KEY (content_unit_id, tag_id)
);

-- rambler down

DROP TABLE IF EXISTS content_units_tags;