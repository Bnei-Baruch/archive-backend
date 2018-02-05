-- rambler up

ALTER TABLE files ALTER COLUMN sha1 DROP NOT NULL;

-- rambler down

ALTER TABLE files ALTER COLUMN sha1 SET NOT NULL;
