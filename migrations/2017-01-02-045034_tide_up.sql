-- rambler up

ALTER TABLE strings RENAME TO string_translations;

-- rambler down

ALTER TABLE string_translations RENAME TO strings;
