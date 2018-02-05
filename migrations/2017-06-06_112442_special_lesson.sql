-- MDB generated migration file
-- rambler up

UPDATE content_types SET name = 'SPECIAL_LESSON' WHERE name = 'SATURDAY_LESSON';

-- rambler down

UPDATE content_types SET name = 'SATURDAY_LESSON' WHERE name = 'SPECIAL_LESSON';