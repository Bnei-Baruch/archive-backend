-- rambler up

---------------
-- Functions --
---------------

DROP FUNCTION IF EXISTS now_utc();

CREATE FUNCTION now_utc()
  RETURNS TIMESTAMP AS $$
SELECT now() AT TIME ZONE 'utc';
$$ LANGUAGE SQL;

------------
-- Tables --
------------

DROP TABLE IF EXISTS users;
CREATE TABLE users (
  id         BIGSERIAL PRIMARY KEY,
  email      VARCHAR(64) UNIQUE                         NOT NULL,
  name       CHAR(32)                                   NULL,
  phone      VARCHAR(32)                                NULL,
  comments   VARCHAR(255)                               NULL,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT now_utc() NOT NULL,
  updated_at TIMESTAMP WITH TIME ZONE                   NULL,
  deleted_at TIMESTAMP WITH TIME ZONE                   NULL
);


DROP TABLE IF EXISTS strings;
CREATE TABLE strings (
  id                BIGSERIAL UNIQUE,
  language          CHAR(2),
  text              TEXT                                       NOT NULL,
  original_language CHAR(2)                                    NULL,
  user_id           BIGINT                                     NULL,
  created_at        TIMESTAMP WITH TIME ZONE DEFAULT now_utc() NOT NULL,
  PRIMARY KEY (id, language)
);


DROP TABLE IF EXISTS operation_types;
CREATE TABLE operation_types (
  id          BIGSERIAL PRIMARY KEY,
  name        VARCHAR(32) UNIQUE      NOT NULL,
  description VARCHAR(255)            NULL
);


DROP TABLE IF EXISTS operations;
CREATE TABLE operations (
  id         BIGSERIAL PRIMARY KEY,
  uid        CHAR(8) UNIQUE                             NOT NULL,
  type_id    BIGINT REFERENCES operation_types          NOT NULL,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT now_utc() NOT NULL,
  station    VARCHAR(255)                               NULL,
  user_id    BIGINT REFERENCES users                    NULL,
  details    VARCHAR(255)                               NULL
);


DROP TABLE IF EXISTS content_types;
CREATE TABLE content_types (
  id          BIGSERIAL PRIMARY KEY,
  name        VARCHAR(32) UNIQUE      NOT NULL,
  description VARCHAR(255)            NULL
);


DROP TABLE IF EXISTS collections;
CREATE TABLE collections (
  id          BIGSERIAL PRIMARY KEY,
  uid         CHAR(8) UNIQUE                                  NOT NULL,
  type_id     BIGINT REFERENCES content_types                 NOT NULL,
  name        BIGINT REFERENCES strings (id)                  NOT NULL,
  description BIGINT REFERENCES strings (id)                  NULL,
  created_at  TIMESTAMP WITH TIME ZONE DEFAULT now_utc()      NOT NULL,
  properties  JSONB                                           NULL,
  external_id VARCHAR(255) UNIQUE                             NULL
);


DROP TABLE IF EXISTS content_units;
CREATE TABLE content_units (
  id          BIGSERIAL PRIMARY KEY,
  uid         CHAR(8) UNIQUE                                  NOT NULL,
  type_id     BIGINT REFERENCES content_types                 NOT NULL,
  name        BIGINT REFERENCES strings (id)                  NOT NULL,
  description BIGINT REFERENCES strings (id)                  NULL,
  created_at  TIMESTAMP WITH TIME ZONE DEFAULT now_utc()      NOT NULL,
  properties  JSONB                                           NULL
);


DROP TABLE IF EXISTS collections_content_units;
CREATE TABLE collections_content_units (
  collection_id   BIGINT REFERENCES collections   NOT NULL,
  content_unit_id BIGINT REFERENCES content_units NOT NULL,
  name            VARCHAR(255)                    NOT NULL,
  PRIMARY KEY (collection_id, content_unit_id)
);


DROP TABLE IF EXISTS files;
CREATE TABLE files (
  id                BIGSERIAL PRIMARY KEY,
  uid               CHAR(8) UNIQUE                                  NOT NULL,
  name              VARCHAR(255)                                    NOT NULL, -- physical file name
  size              BIGINT                                          NOT NULL, -- physical size in bytes
  type              VARCHAR(16)                                     NOT NULL, -- audio, video, image, text
  sub_type          VARCHAR(16)                                     NOT NULL, -- drawing, photo, song, lesson
  mime_type         VARCHAR(255)                                    NULL,
  sha1              BYTEA UNIQUE                                    NOT NULL,
  operation_id      BIGINT REFERENCES operations                    NULL, -- operation that created the file.
  content_unit_id   BIGINT REFERENCES content_units                 NULL,
  created_at        TIMESTAMP WITH TIME ZONE DEFAULT now_utc()      NOT NULL,
  language          CHAR(2)                                         NULL,
  --   mm_duration       INTEGER                                        NULL,  -- multimedia playing time in seconds, should be in properties field
  --   vid_internal_id   VARCHAR(64)                                    NULL,  -- needs discussion with Amnon & Shaul maybe put in properties field
  backup_count      SMALLINT DEFAULT 0                              NULL, -- number of existing backups
  first_backup_time TIMESTAMP WITH TIME ZONE                        NULL,
  properties        JSONB                                           NULL
);


DROP TABLE IF EXISTS persons;
CREATE TABLE persons (
  id          BIGSERIAL PRIMARY KEY,
  uid         CHAR(8) UNIQUE                 NOT NULL,
  name        BIGINT REFERENCES strings (id) NOT NULL,
  description BIGINT REFERENCES strings (id) NULL
);


DROP TABLE IF EXISTS content_roles;
CREATE TABLE content_roles (
  id          BIGSERIAL PRIMARY KEY,
  name        BIGINT REFERENCES strings (id) NOT NULL,
  description BIGINT REFERENCES strings (id) NULL
);


DROP TABLE IF EXISTS content_units_persons;
CREATE TABLE content_units_persons (
  content_unit_id BIGINT REFERENCES content_units       NOT NULL,
  person_id       BIGINT REFERENCES persons             NOT NULL,
  role_id         BIGINT REFERENCES content_roles       NOT NULL,
  PRIMARY KEY (content_unit_id, person_id)
);


DROP TABLE IF EXISTS tags;
CREATE TABLE tags (
  id          BIGSERIAL PRIMARY KEY,
  label       BIGINT REFERENCES strings (id)    NOT NULL,
  description VARCHAR(255)                      NULL,
  parent_id   BIGINT REFERENCES tags            NULL
);


-------------
-- Indexes --
-------------

CREATE INDEX IF NOT EXISTS operations_created_at_idx
  ON operations USING BTREE (created_at);

CREATE INDEX IF NOT EXISTS strings_created_at_idx
  ON strings USING BTREE (created_at);

CREATE INDEX IF NOT EXISTS collections_created_at_idx
  ON collections USING BTREE (created_at);

CREATE INDEX IF NOT EXISTS content_units_created_at_idx
  ON content_units USING BTREE (created_at);

CREATE INDEX IF NOT EXISTS files_created_at_idx
  ON files USING BTREE (created_at);

CREATE INDEX IF NOT EXISTS files_type_sub_type_idx
  ON files USING BTREE (type, sub_type);


-- rambler down
-- TODO: Check schema is fully cleared.

DROP INDEX IF EXISTS files_type_sub_type_idx;
DROP INDEX IF EXISTS files_created_at_idx;
DROP INDEX IF EXISTS content_units_created_at_idx;
DROP INDEX IF EXISTS collections_created_at_idx;
DROP INDEX IF EXISTS strings_created_at_idx;
DROP INDEX IF EXISTS operations_created_at_idx;

DROP TABLE IF EXISTS tags;
DROP TABLE IF EXISTS content_units_persons;
DROP TABLE IF EXISTS files;
DROP TABLE IF EXISTS collections_content_units;
DROP TABLE IF EXISTS collections;
DROP TABLE IF EXISTS operations;
DROP TABLE IF EXISTS operation_types;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS content_types CASCADE;
DROP TABLE IF EXISTS content_units CASCADE;
DROP TABLE IF EXISTS persons CASCADE;
DROP TABLE IF EXISTS content_roles CASCADE;
DROP TABLE IF EXISTS strings CASCADE;
DROP FUNCTION IF EXISTS now_utc();
