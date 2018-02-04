-- MDB generated migration file
-- rambler up

WITH data(name) AS (VALUES
  -- Collection Types
  ('VIRTUAL_LESSONS'),
  ('CHILDREN_LESSONS'),
  ('WOMEN_LESSONS')
)
INSERT INTO content_types (name)
  SELECT d.name
  FROM data AS d
  WHERE NOT EXISTS(SELECT ct.name
                   FROM content_types AS ct
                   WHERE ct.name = d.name);

UPDATE collections
SET type_id = (SELECT id
               FROM content_types
               WHERE name = 'VIRTUAL_LESSONS')
WHERE type_id = 16;

UPDATE content_types SET name = 'CHILDREN_LESSON' WHERE name = 'CHILDREN_LESSON_PART';
UPDATE content_types SET name = 'WOMEN_LESSON' WHERE name = 'WOMEN_LESSON_PART';

-- rambler down

UPDATE content_types SET name = 'CHILDREN_LESSON_PART' WHERE name = 'CHILDREN_LESSON';
UPDATE content_types SET name = 'WOMEN_LESSON_PART' WHERE name = 'WOMEN_LESSON';

UPDATE collections
SET type_id = 16
WHERE type_id = (SELECT id
                 FROM content_types
                 WHERE name = 'VIRTUAL_LESSONS');

DELETE FROM content_types
WHERE name IN
      ('VIRTUAL_LESSONS', 'CHILDREN_LESSONS', 'WOMEN_LESSONS');