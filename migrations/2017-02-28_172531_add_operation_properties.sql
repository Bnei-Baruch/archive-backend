-- MDB generated migration file
-- rambler up

ALTER TABLE operations
  ADD COLUMN properties JSONB NULL;

-- rambler down

ALTER TABLE operations
  DROP COLUMN properties;
