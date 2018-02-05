-- MDB generated migration file
-- rambler up

DROP TABLE IF EXISTS source_types;
CREATE TABLE source_types (
  id   BIGSERIAL PRIMARY KEY,
  name VARCHAR(32) UNIQUE NOT NULL
);

INSERT INTO source_types (name) VALUES
  ('COLLECTION'),
  ('BOOK'),
  ('VOLUME'),
  ('PART'),
  ('PARASHA'),
  ('CHAPTER'),
  ('ARTICLE'),
  ('TITLE'),
  ('LETTER'),
  ('ITEM');

DROP TABLE IF EXISTS sources;
CREATE TABLE sources (
  id          BIGSERIAL PRIMARY KEY,
  uid         CHAR(8) UNIQUE                                  NOT NULL,
  parent_id   BIGINT REFERENCES sources                       NULL,
  pattern     VARCHAR(255) UNIQUE                             NULL,
  type_id     BIGINT REFERENCES source_types                  NOT NULL,
  position    INTEGER                                         NULL,
  name        VARCHAR(255)                                    NOT NULL,
  description TEXT                                            NULL,
  created_at  TIMESTAMP WITH TIME ZONE DEFAULT now_utc()      NOT NULL,
  properties  JSONB                                           NULL
);

DROP TABLE IF EXISTS source_i18n;
CREATE TABLE source_i18n (
  source_id   BIGINT REFERENCES sources (id)                                   NOT NULL,
  language    CHAR(2)                                                          NOT NULL,
  name        VARCHAR(255)                                                     NULL,
  description TEXT                                                             NULL,
  created_at  TIMESTAMP WITH TIME ZONE DEFAULT now_utc()                       NOT NULL,
  PRIMARY KEY (source_id, language)
);

DROP TABLE IF EXISTS authors;
CREATE TABLE authors (
  id         BIGSERIAL PRIMARY KEY,
  code       CHAR(2) UNIQUE                                                   NOT NULL,
  name       VARCHAR(255)                                                     NOT NULL,
  full_name  VARCHAR(255)                                                     NULL,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT now_utc()                       NOT NULL
);

DROP TABLE IF EXISTS author_i18n;
CREATE TABLE author_i18n (
  author_id  BIGINT REFERENCES authors (id)                                   NOT NULL,
  language   CHAR(2)                                                          NOT NULL,
  name       VARCHAR(255)                                                     NULL,
  full_name  VARCHAR(255)                                                     NULL,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT now_utc()                       NOT NULL,
  PRIMARY KEY (author_id, language)
);

DROP TABLE IF EXISTS authors_sources;
CREATE TABLE authors_sources (
  author_id BIGINT REFERENCES authors       NOT NULL,
  source_id BIGINT REFERENCES sources       NOT NULL,
  PRIMARY KEY (author_id, source_id)
);

DROP TABLE IF EXISTS content_units_sources;
CREATE TABLE content_units_sources (
  content_unit_id BIGINT REFERENCES content_units NOT NULL,
  source_id       BIGINT REFERENCES sources       NOT NULL,
  PRIMARY KEY (content_unit_id, source_id)
);

CREATE INDEX IF NOT EXISTS sources_parent_id_idx
  ON sources USING BTREE (parent_id);

-- rambler down

DROP TABLE IF EXISTS content_units_sources;
DROP TABLE IF EXISTS authors_sources;
DROP TABLE IF EXISTS author_i18n;
DROP TABLE IF EXISTS authors;
DROP TABLE IF EXISTS source_i18n;
DROP TABLE IF EXISTS sources;
DROP TABLE IF EXISTS source_types;



