-- MDB generated migration file
-- rambler up

ALTER TABLE collections_content_units
  ADD COLUMN position INT NOT NULL DEFAULT 0;

UPDATE collections_content_units
SET position = CAST((COALESCE(NULLIF(REGEXP_REPLACE(name, '[^0-9]+', '', 'g'), ''), '0')) AS INTEGER);

-- rambler down

ALTER TABLE collections_content_units
  DROP COLUMN position;
