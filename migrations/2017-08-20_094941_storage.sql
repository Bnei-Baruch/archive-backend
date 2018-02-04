-- MDB generated migration file
-- rambler up

DROP TABLE IF EXISTS storages;
CREATE TABLE storages (
  id       BIGSERIAL PRIMARY KEY,
  name     VARCHAR(255) UNIQUE NOT NULL,
  country  CHAR(2)             NOT NULL,
  location VARCHAR(30)         NOT NULL,
  status   VARCHAR(30)         NOT NULL,
  access   VARCHAR(30)         NOT NULL
);

DROP TABLE IF EXISTS files_storages;
CREATE TABLE files_storages (
  file_id    BIGINT REFERENCES files                          NOT NULL,
  storage_id BIGINT REFERENCES storages ON DELETE CASCADE     NOT NULL,
  PRIMARY KEY (file_id, storage_id)
);

CREATE INDEX IF NOT EXISTS files_storages_storage_id_idx
  ON files_storages USING BTREE (storage_id);

-- rambler down

DROP INDEX IF EXISTS files_storages_storage_id_idx;
DROP TABLE IF EXISTS files_storages;
DROP TABLE IF EXISTS storages;