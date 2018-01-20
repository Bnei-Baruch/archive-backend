
-- MDB generated migration file
-- rambler up

WITH data(name) AS (VALUES
  -- Collection Types
  ('DAILY_LESSON'),
  ('SATURDAY_LESSON'),
  ('WEEKLY_FRIENDS_GATHERING'),
  ('CONGRESS'),
  ('VIDEO_PROGRAM'),
  ('LECTURE_SERIES'),
  ('MEALS'),
  ('HOLIDAY'),
  ('PICNIC'),
  ('UNITY_DAY'),

  -- Content Unit Types
  ('LESSON_PART'),
  ('LECTURE'),
  ('CHILDREN_LESSON_PART'),
  ('WOMEN_LESSON_PART'),
  ('CAMPUS_LESSON'),
  ('LC_LESSON'),
  ('VIRTUAL_LESSON'),
  ('FRIENDS_GATHERING'),
  ('MEAL'),
  ('VIDEO_PROGRAM_CHAPTER'),
  ('FULL_LESSON'),
  ('TEXT'))
INSERT INTO content_types (name) 
SELECT d.name FROM data AS d
WHERE NOT EXISTS (SELECT ct.name FROM content_types AS ct WHERE ct.name = d.name);

-- rambler down

DELETE FROM content_types WHERE id IN
  -- Collection Types
  ('DAILY_LESSON',
  'SATURDAY_LESSON',
  'WEEKLY_FRIENDS_GATHERING',
  'CONGRESS',
  'VIDEO_PROGRAM',
  'LECTURE_SERIES',
  'MEALS',
  'HOLIDAY',
  'PICNIC',
  'UNITY_DAY',

  -- Content Unit Types
  'LESSON_PART',
  'LECTURE',
  'CHILDREN_LESSON_PART',
  'WOMEN_LESSON_PART',
  'CAMPUS_LESSON',
  'LC_LESSON',
  'VIRTUAL_LESSON',
  'FRIENDS_GATHERING',
  'MEAL',
  'VIDEO_PROGRAM_CHAPTER',
  'FULL_LESSON',
  'TEXT')
