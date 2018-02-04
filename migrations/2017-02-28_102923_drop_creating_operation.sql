-- MDB generated migration file
-- rambler up

ALTER TABLE files
  DROP COLUMN operation_id;

-- rambler down

ALTER TABLE files
  ADD COLUMN operation_id BIGINT REFERENCES operations NULL;
