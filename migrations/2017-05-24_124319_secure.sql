-- MDB generated migration file
-- rambler up

-- Secure

ALTER TABLE collections
  ADD COLUMN secure SMALLINT NOT NULL DEFAULT 0,
  ADD COLUMN published BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE content_units
  ADD COLUMN secure SMALLINT NOT NULL DEFAULT 0,
  ADD COLUMN published BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE files
  ADD COLUMN secure SMALLINT NOT NULL DEFAULT 0,
  ADD COLUMN published BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE collections
SET secure = coalesce(properties ->> 'secure', '0') :: SMALLINT;

UPDATE content_units
SET secure = coalesce(properties ->> 'secure', '0') :: SMALLINT;

UPDATE files
SET secure = coalesce(properties ->> 'secure', '0') :: SMALLINT;

-- Map kmedia secure levels to 100+ range if not public
-- We're restructuring these levels in the new archive
UPDATE collections
SET secure = 100 + secure
WHERE secure > 0;

UPDATE content_units
SET secure = 100 + secure
WHERE secure > 0;

UPDATE files
SET secure = 100 + secure
WHERE secure > 0;

-- Published

UPDATE files
SET published = coalesce(properties ? 'url', FALSE);

UPDATE content_units
SET published = TRUE
WHERE id IN (SELECT DISTINCT content_unit_id
             FROM files
             WHERE published IS TRUE);

UPDATE collections
SET published = TRUE
WHERE id IN (SELECT DISTINCT ccu.collection_id
             FROM collections_content_units ccu INNER JOIN content_units cu
                 ON ccu.content_unit_id = cu.id AND cu.published IS TRUE);

-- rambler down
ALTER TABLE collections
  DROP COLUMN secure,
  DROP COLUMN published;

ALTER TABLE content_units
  DROP COLUMN secure,
  DROP COLUMN published;

ALTER TABLE files
  DROP COLUMN secure,
  DROP COLUMN published;