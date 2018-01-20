-- MDB generated migration file
-- rambler up

WITH data(name) AS (VALUES
  ('PUBLICATION'),
  ('ARTICLES')
)
INSERT INTO content_types (name)
  SELECT d.name
  FROM data AS d
  WHERE NOT EXISTS(SELECT ct.name
                   FROM content_types AS ct
                   WHERE ct.name = d.name);

UPDATE content_types
SET name = 'ARTICLE'
WHERE name = 'TEXT';

DROP TABLE IF EXISTS publishers;
CREATE TABLE publishers (
  id      BIGSERIAL PRIMARY KEY,
  uid     CHAR(8) UNIQUE     NOT NULL,
  pattern VARCHAR(30) UNIQUE NULL
);

DROP TABLE IF EXISTS publisher_i18n;
CREATE TABLE publisher_i18n (
  publisher_id      BIGINT REFERENCES publishers (id)             NOT NULL,
  language          CHAR(2)                                       NOT NULL,
  original_language CHAR(2)                                       NULL,
  name              TEXT,
  description       TEXT,
  user_id           BIGINT REFERENCES users (id)                  NULL,
  created_at        TIMESTAMP WITH TIME ZONE DEFAULT now_utc()    NOT NULL,
  PRIMARY KEY (publisher_id, language)
);

DROP TABLE IF EXISTS content_units_publishers;
CREATE TABLE content_units_publishers (
  content_unit_id BIGINT REFERENCES content_units               NOT NULL,
  publisher_id    BIGINT REFERENCES publishers                  NOT NULL,
  PRIMARY KEY (content_unit_id, publisher_id)
);

-- rambler down

DROP TABLE IF EXISTS content_units_publishers;
DROP TABLE IF EXISTS publisher_i18n;
DROP TABLE IF EXISTS publishers;

UPDATE content_types
SET name = 'TEXT'
WHERE name = 'ARTICLE';

DELETE FROM content_types
WHERE name IN ('PUBLICATION', 'ARTICLES');

