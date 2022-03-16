-- MDB generated migration file
-- rambler up

ALTER TABLE users
    ALTER COLUMN name TYPE VARCHAR(255),
    ADD COLUMN account_id VARCHAR(36);

-- rambler down
ALTER TABLE users
    ALTER COLUMN name TYPE VARCHAR(32),
    DROP COLUMN account_id;
