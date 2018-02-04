-- rambler up

ALTER TABLE collections RENAME COLUMN name to name_id;
ALTER TABLE collections RENAME COLUMN description to description_id;

ALTER TABLE content_units RENAME COLUMN name to name_id;
ALTER TABLE content_units RENAME COLUMN description to description_id;

ALTER TABLE persons RENAME COLUMN name to name_id;
ALTER TABLE persons RENAME COLUMN description to description_id;

ALTER TABLE content_roles RENAME COLUMN name to name_id;
ALTER TABLE content_roles RENAME COLUMN description to description_id;

ALTER TABLE tags RENAME COLUMN label to label_id;

-- rambler down

ALTER TABLE collections RENAME COLUMN name_id to name;
ALTER TABLE collections RENAME COLUMN description_id to description;

ALTER TABLE content_units RENAME COLUMN name_id to name;
ALTER TABLE content_units RENAME COLUMN description_id to description;

ALTER TABLE persons RENAME COLUMN name_id to name;
ALTER TABLE persons RENAME COLUMN description_id to description;

ALTER TABLE content_roles RENAME COLUMN name_id to name;
ALTER TABLE content_roles RENAME COLUMN description_id to description;

ALTER TABLE tags RENAME COLUMN label_id to label;