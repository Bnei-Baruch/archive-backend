-- MDB generated migration file
-- rambler up

ALTER TABLE tags
  ADD COLUMN uid CHAR(8) UNIQUE   NOT NULL,
  ADD COLUMN pattern VARCHAR(255) NULL;

ALTER TABLE tags_i18n
  RENAME TO tag_i18n;

-- rambler down

ALTER TABLE tags
  DROP COLUMN uid,
  DROP COLUMN pattern;

ALTER TABLE tag_i18n
  RENAME TO tags_i18n;