-- MDB generated migration file
-- rambler up

ALTER TABLE files
  ADD COLUMN removed_at TIMESTAMP WITH TIME ZONE NULL DEFAULT NULL;

-- rambler down

ALTER TABLE files
  DROP COLUMN removed_at;