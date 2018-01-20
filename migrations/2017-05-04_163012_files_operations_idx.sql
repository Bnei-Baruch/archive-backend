-- MDB generated migration file
-- rambler up

CREATE INDEX IF NOT EXISTS files_operations_operation_id_idx
  ON files_operations USING BTREE (operation_id);

-- rambler down

DROP INDEX IF EXISTS files_operations_operation_id_idx;
