-- rambler up

ALTER TABLE files ADD parent_id BIGINT REFERENCES files NULL;

-- rambler down

ALTER TABLE files DROP COLUMN parent_id
