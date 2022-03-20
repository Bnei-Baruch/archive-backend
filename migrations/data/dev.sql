-- rambler up
INSERT INTO users (email, account_id)
VALUES
    -- Operator for operations.
    ('operator1@dev.com', 'operator1@dev.com'),
    ('operator2@dev.com', 'operator2@dev.com'),
    ('operator3@dev.com', 'operator3@dev.com'),
    ('operator@dev.com', 'operator@dev.com');


DROP TABLE IF EXISTS batch_convert;
CREATE TABLE batch_convert (
  file_id       BIGINT REFERENCES files                          NOT NULL,
  operation_id  BIGINT REFERENCES operations                     NULL,
  request_at    TIMESTAMP WITH TIME ZONE                         NULL,
  request_error TEXT                                             NULL
);