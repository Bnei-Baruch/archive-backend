-- MDB generated migration file
-- rambler up

UPDATE collections SET type_id = 2 WHERE type_id = 23;

DELETE FROM content_types
WHERE name IN (
  'CAMPUS_LESSON',
  'VIRTUAL_LESSON',
  'WEEKLY_FRIENDS_GATHERING');


-- run manually on production db only before applying this migration
-- DELETE FROM content_types WHERE name = 'PICNIC';
-- DELETE FROM content_types WHERE name = 'FRIENDS_GATHERING';
-- DELETE FROM content_types WHERE name = 'SATURDAY_LESSON';
-- UPDATE content_types SET name = 'SATURDAY_LESSON' WHERE name = 'SHABBAT_LESSON';

UPDATE content_types SET name = 'FRIENDS_GATHERINGS' WHERE name = 'WEEKLY_YH';
UPDATE content_types SET name = 'FRIENDS_GATHERING' WHERE name = 'YESHIVAT_HAVERIM';
UPDATE content_types SET name = 'PICNIC' WHERE name = 'PIKNIK';


WITH data(name) AS (VALUES
  ('CLIP'),
  ('TRAINING'),
  ('KITEI_MAKOR'),
  ('FRIENDS_GATHERINGS'))
INSERT INTO content_types (name)
  SELECT d.name
  FROM data AS d
  WHERE NOT EXISTS(SELECT ct.name
                   FROM content_types AS ct
                   WHERE ct.name = d.name);

-- rambler down

-- Sorry, can't really implement this backward migration.