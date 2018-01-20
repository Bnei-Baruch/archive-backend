-- MDB generated migration file
-- rambler up

DROP TABLE IF EXISTS content_unit_derivations;
CREATE TABLE content_unit_derivations (
  source_id     BIGINT REFERENCES content_units   NOT NULL,
  derived_id BIGINT REFERENCES content_units   NOT NULL,
  name          VARCHAR(255)                      NOT NULL,
  PRIMARY KEY (source_id, derived_id)
);

-- rambler down

DROP TABLE IF EXISTS content_unit_derivations;