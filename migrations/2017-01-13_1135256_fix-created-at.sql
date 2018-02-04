
-- MDB generated migration file
-- rambler up

ALTER TABLE files ALTER COLUMN created_at SET DEFAULT now_utc();
ALTER TABLE files ADD COLUMN file_created_at TIMESTAMP WITH TIME ZONE NULL;

-- rambler down

ALTER TABLE files ALTER created_at DROP DEFAULT;
ALTER TABLE files DROP file_created_at;
