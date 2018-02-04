-- MDB generated migration file
-- rambler up

WITH data(name) AS (VALUES ('sirtutim'))
INSERT INTO operation_types (name)
  SELECT d.name
  FROM data AS d
  WHERE NOT EXISTS(SELECT ot.name
                   FROM operation_types AS ot
                   WHERE ot.name = d.name);

-- rambler down

DELETE FROM operation_types
WHERE id IN
      ('sirtutim')
