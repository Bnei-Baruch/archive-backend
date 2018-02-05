-- MDB generated migration file
-- rambler up

DROP TABLE IF EXISTS persons;
CREATE TABLE persons (
  id      BIGSERIAL PRIMARY KEY,
  uid     CHAR(8) UNIQUE     NOT NULL,
  pattern VARCHAR(30) UNIQUE NULL
);

DROP TABLE IF EXISTS person_i18n;
CREATE TABLE person_i18n (
  person_id         BIGINT REFERENCES persons (id)             NOT NULL,
  language          CHAR(2)                                    NOT NULL,
  original_language CHAR(2)                                    NULL,
  name              TEXT,
  description       TEXT,
  user_id           BIGINT REFERENCES users (id)               NULL,
  created_at        TIMESTAMP WITH TIME ZONE DEFAULT now_utc() NOT NULL,
  PRIMARY KEY (person_id, language)
);

DROP TABLE IF EXISTS content_role_types;
CREATE TABLE content_role_types (
  id          BIGSERIAL PRIMARY KEY,
  name        VARCHAR(32) UNIQUE NOT NULL,
  description VARCHAR(255)       NULL
);

INSERT INTO content_role_types (name) VALUES
  ('LECTURER');


DROP TABLE IF EXISTS content_units_persons;
CREATE TABLE content_units_persons (
  content_unit_id BIGINT REFERENCES content_units            NOT NULL,
  person_id       BIGINT REFERENCES persons                  NOT NULL,
  role_id         BIGINT REFERENCES content_role_types       NOT NULL,
  PRIMARY KEY (content_unit_id, person_id)
);

DO $$
DECLARE pid BIGINT;
BEGIN
  INSERT INTO persons (uid, pattern) VALUES ('abcdefgh', 'rav')
  RETURNING id
    INTO pid;

  INSERT INTO person_i18n (person_id, language, name) VALUES (pid, 'he', 'מיכאל לייטמן');
  INSERT INTO person_i18n (person_id, language, name) VALUES (pid, 'en', 'Michael Laitman');
  INSERT INTO person_i18n (person_id, language, name) VALUES (pid, 'ru', 'Михаэль Лайтман');
END $$;

-- rambler down

DROP TABLE IF EXISTS content_units_persons;
DROP TABLE IF EXISTS content_role_types;
DROP TABLE IF EXISTS person_i18n;
DROP TABLE IF EXISTS persons;