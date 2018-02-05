
-- MDB generated migration file
-- rambler up

DROP TABLE IF EXISTS files_operations;
CREATE TABLE files_operations (
  file_id   BIGINT REFERENCES files   NOT NULL,
  operation_id BIGINT REFERENCES operations NOT NULL,
  PRIMARY KEY (file_id, operation_id)
);

ALTER TABLE files ALTER created_at DROP DEFAULT;

-- rambler down

DROP TABLE IF EXISTS files_operations;
ALTER TABLE files ALTER COLUMN created_at SET DEFAULT now_utc()
