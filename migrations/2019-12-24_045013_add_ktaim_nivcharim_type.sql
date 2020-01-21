-- MDB generated migration file
-- rambler up

WITH data(name) AS (VALUES
  ('KTAIM_NIVCHARIM')
)
INSERT INTO content_types (name)
  SELECT d.name
  FROM data AS d
  WHERE NOT EXISTS(SELECT ct.name
                   FROM content_types AS ct
                   WHERE ct.name = d.name);

-- rambler down

DELETE FROM content_types
WHERE name IN ('KTAIM_NIVCHARIM');