-- MDB generated migration file
-- rambler up

UPDATE content_types SET name = 'VIRTUAL_LESSON' WHERE name = 'LC_LESSON';

-- rambler down

UPDATE content_types SET name = 'LC_LESSON' WHERE name = 'VIRTUAL_LESSON';
