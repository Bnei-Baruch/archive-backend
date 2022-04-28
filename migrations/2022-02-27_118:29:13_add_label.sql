-- rambler up

DROP TABLE IF EXISTS labels;
CREATE TABLE labels
(
    id              BIGSERIAL PRIMARY KEY,
    uid             CHAR(8) UNIQUE                                    NOT NULL,
    content_unit_id BIGINT REFERENCES content_units ON DELETE CASCADE NOT NULL,
    media_type      VARCHAR(16)                                       NOT NULL,
    properties      JSONB                                             NULL,
    approve_state   SMALLINT                                          NOT NULL DEFAULT 0,
    created_at      TIMESTAMP WITH TIME ZONE                          NOT NULL DEFAULT now_utc()
);


DROP TABLE IF EXISTS label_tag;
CREATE TABLE label_tag
(
    label_id BIGINT REFERENCES labels ON DELETE CASCADE NOT NULL,
    tag_id   BIGINT REFERENCES tags ON DELETE CASCADE   NOT NULL,
    PRIMARY KEY (label_id, tag_id)
);


DROP TABLE IF EXISTS label_i18n;
CREATE TABLE label_i18n
(
    label_id   BIGINT REFERENCES labels (id) ON DELETE CASCADE NOT NULL,
    language   CHAR(2)                                         NOT NULL,
    name       TEXT,
    user_id    BIGINT REFERENCES users                         NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT now_utc()      NOT NULL,
    PRIMARY KEY (label_id, language)
);


-- rambler down
DROP TABLE IF EXISTS label_i18n;
DROP TABLE IF EXISTS label_tag;
DROP TABLE IF EXISTS labels;