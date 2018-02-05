-- MDB generated migration file
-- rambler up

ALTER TABLE "collections"
  DROP CONSTRAINT IF EXISTS "collections_name_fkey";
ALTER TABLE "collections"
  DROP CONSTRAINT IF EXISTS "collections_description_fkey";
ALTER TABLE "content_roles"
  DROP CONSTRAINT IF EXISTS "content_roles_description_fkey";
ALTER TABLE "content_roles"
  DROP CONSTRAINT IF EXISTS "content_roles_name_fkey";
ALTER TABLE "content_units"
  DROP CONSTRAINT IF EXISTS "content_units_description_fkey";
ALTER TABLE "content_units"
  DROP CONSTRAINT IF EXISTS "content_units_name_fkey";
ALTER TABLE "persons"
  DROP CONSTRAINT IF EXISTS "persons_description_fkey";
ALTER TABLE "persons"
  DROP CONSTRAINT IF EXISTS "persons_name_fkey";
ALTER TABLE "tags"
  DROP CONSTRAINT IF EXISTS "tags_label_fkey";

DROP TABLE IF EXISTS string_translations;

CREATE TABLE collection_i18n (
  collection_id     BIGINT REFERENCES collections (id)                             NOT NULL,
  language          CHAR(2)                                                        NOT NULL,
  original_language CHAR(2)                                                        NULL,
  name              TEXT,
  description       TEXT,
  user_id           BIGINT REFERENCES users (id)                                   NULL,
  created_at        TIMESTAMP WITH TIME ZONE DEFAULT now_utc()                     NOT NULL,
  PRIMARY KEY (collection_id, language)
);

CREATE TABLE content_unit_i18n (
  content_unit_id   BIGINT REFERENCES content_units (id)                             NOT NULL,
  language          CHAR(2)                                                          NOT NULL,
  original_language CHAR(2)                                                          NULL,
  name              TEXT,
  description       TEXT,
  user_id           BIGINT REFERENCES users (id)                                     NULL,
  created_at        TIMESTAMP WITH TIME ZONE DEFAULT now_utc()                       NOT NULL,
  PRIMARY KEY (content_unit_id, language)
);

CREATE TABLE tags_i18n (
  tag_id            BIGINT REFERENCES tags (id)                                      NOT NULL,
  language          CHAR(2)                                                          NOT NULL,
  original_language CHAR(2)                                                          NULL,
  label             TEXT,
  user_id           BIGINT REFERENCES users (id)                                     NULL,
  created_at        TIMESTAMP WITH TIME ZONE DEFAULT now_utc()                       NOT NULL,
  PRIMARY KEY (tag_id, language)
);

ALTER TABLE collections
  DROP COLUMN name_id,
  DROP COLUMN description_id,
  DROP COLUMN external_id;

ALTER TABLE content_units
  DROP COLUMN name_id,
  DROP COLUMN description_id;

ALTER TABLE tags
  DROP COLUMN label_id;

-- Remove future predictions
DROP TABLE IF EXISTS content_units_persons;
DROP TABLE IF EXISTS persons;
DROP TABLE IF EXISTS content_roles;

-- rambler down

-- Sorry, can't really implement this backward migration due to backward incompatible changes to schema.