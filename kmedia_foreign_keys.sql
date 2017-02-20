-- foreign keys
ALTER TABLE catalogs
  ADD CONSTRAINT parent_fkey FOREIGN KEY (parent_id) REFERENCES catalogs (id);

ALTER TABLE catalogs
  ADD CONSTRAINT user_fkey FOREIGN KEY (user_id) REFERENCES users (id) NOT VALID;

ALTER TABLE catalog_descriptions
  ADD CONSTRAINT catalog_fkey FOREIGN KEY (catalog_id) REFERENCES catalogs (id) NOT VALID;

ALTER TABLE container_description_patterns
  ADD CONSTRAINT user_fkey FOREIGN KEY (user_id) REFERENCES users (id);

ALTER TABLE container_descriptions
  ADD CONSTRAINT container_fkey FOREIGN KEY (container_id) REFERENCES containers (id) NOT VALID;

ALTER TABLE container_transcripts
  ADD CONSTRAINT container_fkey FOREIGN KEY (container_id) REFERENCES containers (id);

ALTER TABLE containers
  ADD CONSTRAINT lecturer_fkey FOREIGN KEY (lecturer_id) REFERENCES lecturers (id);

ALTER TABLE containers
  ADD CONSTRAINT content_type_fkey FOREIGN KEY (content_type_id) REFERENCES content_types (id);

ALTER TABLE containers
  ADD CONSTRAINT virtual_lesson_fkey FOREIGN KEY (virtual_lesson_id) REFERENCES virtual_lessons (id) NOT VALID;

ALTER TABLE containers
  ADD CONSTRAINT user_fkey FOREIGN KEY (user_id) REFERENCES users (id) NOT VALID;

ALTER TABLE containers
  ADD CONSTRAINT censor_fkey FOREIGN KEY (censor_id) REFERENCES users (id) NOT VALID;

ALTER TABLE dictionary_descriptions
  ADD CONSTRAINT dictionary_fkey FOREIGN KEY (dictionary_id) REFERENCES dictionaries (id);

ALTER TABLE file_asset_descriptions
  ADD CONSTRAINT file_asset_fkey FOREIGN KEY (file_id) REFERENCES file_assets (id) NOT VALID;

ALTER TABLE file_assets
  RENAME asset_type TO asset_type_id;
ALTER TABLE file_assets
  ADD CONSTRAINT file_type_fkey FOREIGN KEY (asset_type) REFERENCES file_types (name) NOT VALID;

ALTER TABLE file_assets
  RENAME servername TO server_name_id;
ALTER TABLE file_assets
  ADD CONSTRAINT server_fkey FOREIGN KEY (servername) REFERENCES servers (servername);

ALTER TABLE file_assets
  ADD CONSTRAINT user_fkey FOREIGN KEY (user_id) REFERENCES users (id) NOT VALID;

ALTER TABLE label_descriptions
  ADD CONSTRAINT label_fkey FOREIGN KEY (label_id) REFERENCES labels (id) NOT VALID;

ALTER TABLE lecturer_descriptions
  ADD CONSTRAINT lecturer_fkey FOREIGN KEY (lecturer_id) REFERENCES lecturers (id);

ALTER TABLE users
  ADD CONSTRAINT department_fkey FOREIGN KEY (department_id) REFERENCES departments (id);

ALTER TABLE virtual_lessons
  ADD CONSTRAINT user_fkey FOREIGN KEY (user_id) REFERENCES users (id);

-- languages
ALTER TABLE languages
  ADD CONSTRAINT unique_code3 UNIQUE (code3);

ALTER TABLE catalog_descriptions
  RENAME lang TO lang_id;
ALTER TABLE containers
  RENAME lang TO lang_id;
ALTER TABLE container_descriptions
  RENAME lang TO lang_id;
ALTER TABLE container_transcripts
  RENAME lang TO lang_id;
ALTER TABLE container_description_patterns
  RENAME lang TO lang_id;
ALTER TABLE dictionary_descriptions
  RENAME lang TO lang_id;
ALTER TABLE file_assets
  RENAME lang TO lang_id;
ALTER TABLE file_asset_descriptions
  RENAME lang TO lang_id;
ALTER TABLE label_descriptions
  RENAME lang TO lang_id;
ALTER TABLE lecturer_descriptions
  RENAME lang TO lang_id;

ALTER TABLE catalog_descriptions
  ADD CONSTRAINT language_fkey FOREIGN KEY (lang_id) REFERENCES languages (code3);
ALTER TABLE containers
  ADD CONSTRAINT language_fkey FOREIGN KEY (lang_id) REFERENCES languages (code3) NOT VALID;
ALTER TABLE container_descriptions
  ADD CONSTRAINT language_fkey FOREIGN KEY (lang_id) REFERENCES languages (code3);
ALTER TABLE container_transcripts
  ADD CONSTRAINT language_fkey FOREIGN KEY (lang_id) REFERENCES languages (code3) NOT VALID;
ALTER TABLE container_description_patterns
  ADD CONSTRAINT language_fkey FOREIGN KEY (lang_id) REFERENCES languages (code3);
ALTER TABLE dictionary_descriptions
  ADD CONSTRAINT language_fkey FOREIGN KEY (lang_id) REFERENCES languages (code3) NOT VALID;
ALTER TABLE file_assets
  ADD CONSTRAINT language_fkey FOREIGN KEY (lang_id) REFERENCES languages (code3) NOT VALID;
ALTER TABLE file_asset_descriptions
  ADD CONSTRAINT language_fkey FOREIGN KEY (lang_id) REFERENCES languages (code3) NOT VALID;
ALTER TABLE label_descriptions
  ADD CONSTRAINT language_fkey FOREIGN KEY (lang_id) REFERENCES languages (code3) NOT VALID;
ALTER TABLE lecturer_descriptions
  ADD CONSTRAINT language_fkey FOREIGN KEY (lang_id) REFERENCES languages (code3);

--
-- ALTER TABLE catalog_descriptions
--   DROP CONSTRAINT language_fkey;
-- ALTER TABLE containers
--   DROP CONSTRAINT language_fkey;
-- ALTER TABLE container_descriptions
--   DROP CONSTRAINT language_fkey;
-- ALTER TABLE container_transcripts
--   DROP CONSTRAINT language_fkey;
-- ALTER TABLE container_description_patterns
--   DROP CONSTRAINT language_fkey;
-- ALTER TABLE dictionary_descriptions
--   DROP CONSTRAINT language_fkey;
-- ALTER TABLE file_assets
--   DROP CONSTRAINT language_fkey;
-- ALTER TABLE file_asset_descriptions
--   DROP CONSTRAINT language_fkey;
-- ALTER TABLE label_descriptions
--   DROP CONSTRAINT language_fkey;
-- ALTER TABLE lecturer_descriptions
--   DROP CONSTRAINT language_fkey;

-- One to Many

ALTER TABLE catalogs_containers
  ADD CONSTRAINT catalog_fkey FOREIGN KEY (catalog_id) REFERENCES catalogs (id) NOT VALID;
ALTER TABLE catalogs_containers
  ADD CONSTRAINT containers_fkey FOREIGN KEY (container_id) REFERENCES containers (id) NOT VALID;


ALTER TABLE containers_labels
  ADD CONSTRAINT containers_labels_pkey PRIMARY KEY (container_id, label_id);
ALTER TABLE containers_labels
  ADD CONSTRAINT containers_fkey FOREIGN KEY (container_id) REFERENCES containers (id) NOT VALID;
ALTER TABLE containers_labels
  ADD CONSTRAINT label_fkey FOREIGN KEY (label_id) REFERENCES labels (id) NOT VALID;


ALTER TABLE containers_file_assets
  ADD CONSTRAINT containers_fkey FOREIGN KEY (container_id) REFERENCES containers (id) NOT VALID;
ALTER TABLE containers_file_assets
  ADD CONSTRAINT file_asset_fkey FOREIGN KEY (file_asset_id) REFERENCES file_assets (id) NOT VALID;


ALTER TABLE catalogs_container_description_patterns
  ADD CONSTRAINT catalogs_container_description_patterns_pkey PRIMARY KEY (catalog_id, container_description_pattern_id);
ALTER TABLE catalogs_container_description_patterns
  ADD CONSTRAINT catalogs_fkey FOREIGN KEY (catalog_id) REFERENCES catalogs (id);
ALTER TABLE catalogs_container_description_patterns
  ADD CONSTRAINT container_description_pattern_fkey FOREIGN KEY (container_description_pattern_id) REFERENCES container_description_patterns (id);

-- Many to Many

-- This will delete some bad data first
SELECT DISTINCT *
INTO ru
FROM roles_users;
DROP TABLE roles_users;
SELECT *
INTO roles_users
FROM ru;
DROP TABLE ru;

ALTER TABLE roles_users
  DROP CONSTRAINT IF EXISTS role_user_pkey;
ALTER TABLE roles_users
  ADD CONSTRAINT role_user_pkey PRIMARY KEY (role_id, user_id);
ALTER TABLE roles_users
  DROP CONSTRAINT IF EXISTS roles_fkey;
ALTER TABLE roles_users
  ADD CONSTRAINT users_fkey FOREIGN KEY (user_id) REFERENCES users (id) NOT VALID;
