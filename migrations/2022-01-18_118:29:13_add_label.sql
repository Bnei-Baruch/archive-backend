-- rambler up

DROP TABLE IF EXISTS labels;
CREATE TABLE labels
(
    id              BIGSERIAL PRIMARY KEY,
    uid             CHAR(8) UNIQUE                             NOT NULL,
    name            VARCHAR(255)                               NOT NULL,
    subject_uid     CHAR(8)                                    NOT NULL,
    subject_type_id BIGINT REFERENCES content_types            NOT NULL,
    media_type      VARCHAR                                    NOT NULL,
    properties      JSONB                                      NULL,
    accepted        BOOLEAN,
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT now_utc() NOT NULL
);


DROP TABLE IF EXISTS label_i18n;
CREATE TABLE label_i18n
(
    label_id     BIGINT REFERENCES labels (id)              NOT NULL,
    language     CHAR(2)                                    NOT NULL,
    name         TEXT,
    created_with VARCHAR                                    NOT NULL,
    created_at   TIMESTAMP WITH TIME ZONE DEFAULT now_utc() NOT NULL,
    PRIMARY KEY (label_id, language)
);


-- rambler down
DROP TABLE IF EXISTS label_i18n;
DROP TABLE IF EXISTS labels;